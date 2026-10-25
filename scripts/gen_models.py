#!/usr/bin/env python3
"""Generate models.go from the API's OpenAPI document.

    curl -sf https://songlinks.io/v1/openapi.json -o testdata/openapi.json
    python3 scripts/gen_models.py && gofmt -w models.go

One struct per response schema, fields in the API's order, each with its JSON tag. A field the API may leave out or
send as null is a pointer (or a nil slice), so "absent" stays distinguishable from zero: `Works *int` that is nil means
the API did not say, not that there are no works. Strings are the exception: null decodes to "". Unknown JSON fields are ignored by encoding/json, so a field the API
adds later never breaks a client built today.

Doc comments are the API's own descriptions, made into Go sentences. A type's comment starts with its name, as godoc
expects, and the field names the API's prose mentions in backticks are given as the Go names. models_test.go checks the
generated file against the same document.
"""
from __future__ import annotations

import json
import os
import re
import textwrap

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SPEC = os.path.join(ROOT, "testdata", "openapi.json")
OUT = os.path.join(ROOT, "models.go")
SITE = "https://songlinks.io"

# The schemas a client reads, and their Go names. Two are renamed, as in the other SDKs: `Record` reads as a generic
# word, and `Error` is the SDK's own error type. Request-side and FastAPI's validation schemas are left out.
NAMES = {
    "LookupResponse": "LookupResponse", "Query": "Query", "Cluster": "Cluster", "Record": "LookupRecord",
    "Evidence": "Evidence", "ReviewItem": "ReviewItem", "Link": "Link", "Rejection": "Rejection",
    "Provenance": "Provenance", "SongMatch": "SongMatch", "SongRecord": "SongRecord", "SongBeyond": "SongBeyond",
    "RateLimit": "RateLimit", "PersonResponse": "PersonResponse", "PersonRecord": "PersonRecord",
    "PersonWork": "PersonWork", "WorksPage": "WorksPage", "PersonEvent": "PersonEvent",
    "PersonProvenance": "PersonProvenance", "BuildRef": "BuildRef",
    "PersonWithdrawnResponse": "PersonWithdrawnResponse", "WritersResponse": "WritersResponse",
    "WriterAnswer": "WriterAnswer", "WritersSummary": "WritersSummary", "VersionResponse": "VersionResponse",
    "Error": "APIError", "RateLimited": "RateLimited",
}

# Go's initialisms (https://go.dev/wiki/CodeReviewComments#initialisms), as they occur in the API's field names.
INITIALISMS = {"id": "ID", "ipi": "IPI", "url": "URL", "iswc": "ISWC", "md5": "MD5", "mlc": "MLC", "ms": "Ms", "api": "API"}

# Where the API's wording names its query parameters, the Go reader needs the method to call. Keyed by (schema, field).
OVERRIDES = {
    # `candidates` is both this field's value and a field name; the general rule would turn the value into the field.
    ("LookupResponse", "answer"): "Either `candidates` or `rejected`. When it is `rejected`, Rejection says why.",
    ("WorksPage", "next_offset"): "The offset to ask for next, as PersonOptions.WorksOffset, or nil on the last page. "
        "Client.PersonWorks reads every page for you.",
    ("PersonResponse", "works_list"): "One page of works. For the next page, pass WorksList.NextOffset as "
        "PersonOptions.WorksOffset, or read them all with Client.PersonWorks.",
}


def go_name(field: str) -> str:
    return "".join(INITIALISMS.get(w, w[:1].upper() + w[1:]) for w in field.split("_"))


def sentence(text: str, fields: dict[str, str], string: bool = False) -> str:
    t = re.sub(r"\s+", " ", str(text)).strip()
    if not t:
        return t
    t = re.sub(r"\[([^\]]+)\]\((/[^)\s]*)\)", lambda m: f"[{m[1]}]({SITE}{m[2]})", t)
    # A backticked name that is a field of this schema is given as the Go field; any other backticked word is a
    # value (`model`, `rejected`) and stays as the API writes it.
    t = re.sub(r"`([a-z0-9_]+)`", lambda m: fields.get(m[1], m[0]), t)
    # A Go reader meets null as nil, or, in a string field, as "".
    word = "empty" if string else "nil"
    t = re.sub(r"\bNull\b", word.capitalize(), re.sub(r"\bnull\b", word, t))
    t = t[0].upper() + t[1:]
    return t if t[-1] in ".!?" else t + "."


def type_comment(name: str, text: str) -> str:
    """Start the comment with the type's name, as godoc expects, keeping the API's sentence."""
    m = re.match(r"HTTP (\d{3}): (.*)", text)
    if m:
        return f"{name} is the body of an HTTP {m[1]} answer: {m[2][0].lower()}{m[2][1:]}"
    first = text.split(" ", 1)[0]
    # Plural nouns read badly after "is": "WritersSummary is totals over the list".
    verb = {"WritersResponse": "lists", "WritersSummary": "holds"}.get(name)
    if verb:
        return f"{name} {verb} {text[0].lower()}{text[1:]}"
    lowered = text[0].lower() + text[1:]
    if first in ("Why", "What", "Which"):
        return f"{name} says {lowered}"
    return f"{name} is {lowered}"


def go_type(prop: dict, optional: bool) -> str:
    if "$ref" in prop:
        inner = NAMES[prop["$ref"].rsplit("/", 1)[-1]]
        return f"*{inner}" if optional else inner
    if "anyOf" in prop:
        kinds = [p for p in prop["anyOf"] if p.get("type") != "null"]
        nullable = len(kinds) < len(prop["anyOf"])
        return go_type(kinds[0], optional or nullable) if len(kinds) == 1 else "json.RawMessage"
    kind = prop.get("type")
    if kind == "array":
        # A nil slice already means absent, so a list is never a pointer.
        return "[]" + go_type(prop.get("items", {}), False)
    if kind == "object":
        return "map[string]any"
    base = {"integer": "int", "number": "float64", "boolean": "bool", "string": "string"}.get(kind, "json.RawMessage")
    # A string is never a pointer: null decodes to "", and no field gives "" a meaning apart from null, so a pointer
    # would only make every name and title harder to print. Numbers and booleans keep theirs, because there nil and
    # zero say different things (Works nil: not known; Prerelease false: a released ID).
    return f"*{base}" if optional and base not in ("json.RawMessage", "string") else base


def comment(text: str, indent: str) -> list[str]:
    return [f"{indent}// {line}" for line in textwrap.wrap(text, 116 - len(indent))]


def render(spec: dict) -> str:
    schemas = spec["components"]["schemas"]
    out = [
        "// Code generated by scripts/gen_models.py from the API's OpenAPI document. DO NOT EDIT.",
        "",
        "package songlinks",
    ]
    for source, name in NAMES.items():
        schema = schemas[source]
        props = schema.get("properties", {})
        fields = {f: go_name(f) for f in props}
        required = set(schema.get("required", []))
        out.append("")
        out += comment(type_comment(name, sentence(schema.get("description") or "", fields)), "")
        out.append(f"type {name} struct {{")
        for i, (field, prop) in enumerate(props.items()):
            if i:
                out.append("")
            optional = field not in required
            kind = go_type(prop, optional)
            text = OVERRIDES.get((source, field)) or sentence(prop.get("description") or "", fields, kind == "string")
            if not text:
                raise SystemExit(f"{source}.{field} has no description in the API's document")
            out += comment(text, "\t")
            tag = f'json:"{field},omitempty"' if optional else f'json:"{field}"'
            out.append(f"\t{fields[field]} {kind} `{tag}`")
        out.append("}")
    if any("json.RawMessage" in line for line in out):
        out[3:3] = ["", 'import "encoding/json"']
    return "\n".join(out) + "\n"


if __name__ == "__main__":
    with open(SPEC) as f:
        text = render(json.load(f))
    with open(OUT, "w") as f:
        f.write(text)
