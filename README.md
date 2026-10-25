# songlinks-go

[![Go Reference](https://pkg.go.dev/badge/github.com/Alexkane/songlinks-go.svg)](https://pkg.go.dev/github.com/Alexkane/songlinks-go)
[![ci](https://github.com/Alexkane/songlinks-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Alexkane/songlinks-go/actions/workflows/ci.yml)
[![licence: MIT](https://img.shields.io/badge/licence-MIT-blue.svg)](LICENSE)

**Find the songwriter behind a name.** The official Go SDK for [Songlinks](https://songlinks.io): look up a songwriter
by name or IPI name number and get the person behind every registration, with their works and ISWCs.

One songwriter is rarely one record. They're registered under different spellings, under several IPI name numbers, or
with no number at all. Songlinks groups those registrations into one person, shows the evidence for every join, and
keeps apart the ones it isn't sure of.

- **Free, with no key and no sign-up.**
- **Typed end to end.** Every answer is a struct generated from the API's own OpenAPI document, with the API's
  description of each field as its doc comment.
- **No dependencies.** The standard library only, on Go 1.23 and later.
- **Built to be called from real apps.** `context` on every call, typed errors, retries that honour `Retry-After`, and
  paging done for you with a range-over-func iterator.

```sh
go get github.com/Alexkane/songlinks-go
```

## Thirty seconds

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Alexkane/songlinks-go"
)

func main() {
	ctx := context.Background()
	client := songlinks.New(songlinks.WithUserAgent("my-app/1.0"))

	answer, err := client.Lookup(ctx, "mark hoppus", nil)
	if err != nil {
		log.Fatal(err)
	}
	best := answer.Candidates[0]
	fmt.Println(best.PersistentID, best.Names, best.Works)
	// SLP-ZYM7-GD3E-S [MARK HOPPUS] 302

	person, err := client.Person(ctx, "SLP-ZYM7-GD3E-S", &songlinks.PersonOptions{WorksLimit: 3})
	if err != nil {
		log.Fatal(err)
	}
	for _, work := range person.WorksList.Items {
		fmt.Println(work.Title) // I MISS YOU, ALL THE SMALL THINGS, DOWN
	}

	check, err := client.Writers(ctx, []string{"00338980326", "00340247397"})
	if err != nil {
		log.Fatal(err)
	}
	for _, w := range check.Writers {
		fmt.Println(w.IPINameNumber, w.PersonID, w.Name)
		// 00338980326 SLP-ZYM7-GD3E-S MARK HOPPUS
		// 00340247397 SLP-MG23-XM9B-Y TRAVIS L BARKER
	}
}
```

Every person has a page you can link to, such as [songlinks.io/person/SLP-ZYM7-GD3E-S](https://songlinks.io/person/SLP-ZYM7-GD3E-S).
It shows the registrations behind the person and what each join rests on.

> **Beta.** Unverified machine output, from a dated snapshot of registration data (every answer's
> `Provenance.MLCSnapshot` gives its date). Some answers are wrong and some are incomplete. Not for rights or royalty
> decisions.

### More

```go
// Which of several writers with the same name wrote a song? Pass one of their songs.
answer, err := client.Lookup(ctx, "amy allen", &songlinks.LookupOptions{Song: "espresso"})
// answer.SongMatch.Match == "exact", answer.SongMatch.Candidates == [c1]: the Amy Allen credited on that song

// Every work of a person, paged for you. Breaking out of the loop stops the requests.
for work, err := range client.PersonWorks(ctx, "SLP-ZYM7-GD3E-S", 0) {
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(work.Title)
}
```

## Checking a whole roster?

The API answers one name at a time, which suits an app. If you administer a catalogue, send us the writers instead.
For each one we send back every registration we can group to them, across spellings, stage names and more than one IPI
name number, with the evidence behind each join and the records we deliberately left out. See
[bulk songwriter lookup](https://songlinks.io/bulk-songwriter-lookup), or write to alex@songlinks.to.

## Examples

Runnable programs for the jobs people use Songlinks for, each under 70 lines. Copy one and change it:

| Program | What it does |
| --- | --- |
| [`resolve-name`](examples/resolve-name/main.go) | Every person a name could be, with their registrations and what joined them. |
| [`which-namesake`](examples/which-namesake/main.go) | Two writers share a name: which one wrote this song? |
| [`check-roster`](examples/check-roster/main.go) | A list of IPI name numbers in, CSV out: who holds each and their works. |
| [`export-catalogue`](examples/export-catalogue/main.go) | Every work of a person, as CSV. |

```sh
go run ./examples/check-roster roster.txt > checked.csv
```

The examples are vetted and built with the package in CI, so one that misreads a field fails the build.

## What it covers

| Method | API | What it answers |
| --- | --- | --- |
| `Lookup(ctx, name, *LookupOptions)` | `GET /v1/lookup` | The people a songwriter's name could be. A `Song` the writer wrote says which of them wrote it. |
| `Person(ctx, id, *PersonOptions)` | `GET /v1/person/{id}` | One person: their registrations, names, IPI name numbers and a page of works. |
| `PersonWorks(ctx, id, pageSize)` | `GET /v1/person/{id}` | Every work of a person, as an `iter.Seq2[PersonWork, error]` over the pages. |
| `Writers(ctx, ipiNameNumbers)` | `GET /v1/writers` | Up to 50 IPI name numbers checked at once: the stored person behind each. |
| `Version(ctx)` | `GET /v1/version` | What is serving: the release, the model and the date of the registration data. |

The answer types are named after the API's schemas (`LookupResponse`, `PersonResponse`, `Cluster`, `WritersResponse`
and the rest), with Go's initialisms (`PersistentID`, `IPINameNumber`, `ISWC`). Two are renamed: the API's `Record` is
`LookupRecord` and its `Error` is `APIError`. The full reference is on
[pkg.go.dev](https://pkg.go.dev/github.com/Alexkane/songlinks-go).

### Reading an answer

- **A missing string is `""`.** Many records have no person ID yet, and that is ordinary. A number, boolean or object
  the API may leave out is a pointer, because there nil and zero say different things: a nil `Works` is not known, and
  a false `Prerelease` is a released ID.
- **A lookup candidate is not a stored person.** It is the lookup's own grouping, made for the name you asked.
  `PersistentID` is the stored person when there is exactly one. A candidate whose records belong to more than one
  stored person has an empty `PersistentID` and lists them in `StoredPersons`. Read each record's own `PersistentID`.
- **Person IDs can be prerelease.** A prerelease ID may stop resolving, or point at a different person, without notice.
  Check `Prerelease` before you store one.

## Errors and retries

An error answer from the API is an `*songlinks.Error` with the HTTP `StatusCode`, the API's `Code` (the one to branch
on), a `Message` for people, and the response `Body`. A withdrawn person ID is a `*songlinks.PersonWithdrawnError`
carrying the API's full answer about the ID; it also unwraps to an `*Error` with `Code` `"withdrawn"`.

```go
_, err := client.Person(ctx, "SLP-0000-0000-0", nil)

var withdrawn *songlinks.PersonWithdrawnError
var apiErr *songlinks.Error
switch {
case errors.As(err, &withdrawn):
	fmt.Println("withdrawn at", withdrawn.Person.WithdrawnAt) // a 410, and a valid answer to keep
case errors.As(err, &apiErr) && apiErr.Code == "rate_limited":
	fmt.Println("retry in", apiErr.RetryAfter)
case errors.As(err, &apiErr):
	fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.Message)
case err != nil:
	log.Fatal(err) // a network failure, after the client's retries
}
```

- **Retried automatically:** `429` and `503` answers and network failures, up to `WithMaxRetries` times (default 2). A
  `Retry-After` header is honoured, up to a minute; otherwise it backs off from half a second.
- **Never retried:** any other error, and a cancelled or expired `context`.
- **Merged person IDs** are followed to the person they now point at, and the answer's `RedirectedFrom` names the ID
  you asked for.

## Options

```go
songlinks.New(
	songlinks.WithBaseURL("https://songlinks.io"),  // the default
	songlinks.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}), // default: 30-second timeout
	songlinks.WithMaxRetries(2),                    // 0 turns retrying off
	songlinks.WithUserAgent("my-app/1.2"),          // appended to songlinks-go/<version>
)
```

A `Client` is safe for concurrent use; create one and share it.

## Fair use

The API is free and open. Please send a `WithUserAgent` that names your app, keep to one request at a time per user
action, and cache answers you reuse. A burst of lookups may be rate-limited; for a whole roster, see
[bulk songwriter lookup](https://songlinks.io/bulk-songwriter-lookup).

## Links

- [API reference](https://songlinks.io/v1/docs), with every endpoint runnable from the page.
- [How Songlinks decides](https://songlinks.io/about): what a person ID is, and how registrations are grouped.
- [MCP server](https://songlinks.io/mcp-server): the same answers inside Claude or any MCP client.
- Something wrong in an answer? [Open an issue](https://github.com/Alexkane/songlinks-go/issues), or write to
  alex@songlinks.to.

## Development

```sh
go vet ./... && go test ./...          # unit tests against a local server, the types against the API's document
SONGLINKS_LIVE=1 go test ./...         # the same calls against songlinks.io
```

The types are generated: `curl -sf https://songlinks.io/v1/openapi.json -o testdata/openapi.json`, then
`python3 scripts/gen_models.py && gofmt -w models.go`. CI regenerates them and fails if `models.go` differs, and
`models_test.go` fails if a type drifts from the document or an exported name loses its doc comment.

## Licence

MIT. See [LICENSE](LICENSE).
