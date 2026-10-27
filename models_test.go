package songlinks

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode"
)

// The generated types against the API's document; real recorded answers against the types; and the doc comments.

var types = map[string]any{
	"LookupResponse": LookupResponse{}, "Query": Query{}, "Cluster": Cluster{}, "Record": LookupRecord{},
	"Evidence": Evidence{}, "ReviewItem": ReviewItem{}, "Link": Link{}, "Rejection": Rejection{},
	"Provenance": Provenance{}, "SongMatch": SongMatch{}, "SongRecord": SongRecord{}, "SongBeyond": SongBeyond{},
	"RateLimit": RateLimit{}, "PersonResponse": PersonResponse{}, "PersonRecord": PersonRecord{},
	"PersonWork": PersonWork{}, "WorksPage": WorksPage{}, "PersonEvent": PersonEvent{},
	"PersonProvenance": PersonProvenance{}, "BuildRef": BuildRef{},
	"PersonWithdrawnResponse": PersonWithdrawnResponse{}, "WritersResponse": WritersResponse{},
	"WriterAnswer": WriterAnswer{}, "WritersSummary": WritersSummary{}, "SongwritersResponse": SongwritersResponse{},
	"RecordingAnswer": RecordingAnswer{}, "RecordingWork": RecordingWork{}, "RecordingWriter": RecordingWriter{},
	"VersionResponse": VersionResponse{}, "Error": APIError{}, "RateLimited": RateLimited{},
}

type schema struct {
	Properties map[string]json.RawMessage `json:"properties"`
	Required   []string                   `json:"required"`
}

func readSchemas(t *testing.T) (map[string]schema, map[string][]string) {
	raw, err := os.ReadFile("testdata/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	schemas, order := map[string]schema{}, map[string][]string{}
	for name, body := range doc.Components.Schemas {
		var s schema
		_ = json.Unmarshal(body, &s)
		schemas[name] = s
		// encoding/json loses key order in a map, so read the property names in order with a token stream.
		order[name] = propertyOrder(t, body)
	}
	return schemas, order
}

func propertyOrder(t *testing.T, body []byte) []string {
	var outer map[string]json.RawMessage
	_ = json.Unmarshal(body, &outer)
	dec := json.NewDecoder(bytes.NewReader(outer["properties"]))
	var names []string
	if _, err := dec.Token(); err != nil {
		return nil
	}
	for dec.More() {
		key, _ := dec.Token()
		names = append(names, key.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return names
}

func TestEveryTypeMatchesItsSchema(t *testing.T) {
	schemas, order := readSchemas(t)
	for source, value := range types {
		typ := reflect.TypeOf(value)
		var tags []string
		for i := range typ.NumField() {
			tag := typ.Field(i).Tag.Get("json")
			name, opts, _ := strings.Cut(tag, ",")
			tags = append(tags, name)
			required := false
			for _, r := range schemas[source].Required {
				required = required || r == name
			}
			if required == (opts == "omitempty") {
				t.Errorf("%s.%s: required %v but tag %q", typ.Name(), name, required, tag)
			}
		}
		if strings.Join(tags, ",") != strings.Join(order[source], ",") {
			t.Errorf("%s fields\n got %v\nwant %v (the API's order)", typ.Name(), tags, order[source])
		}
	}
}

func TestRealAnswersDecodeWithNothingLeftOver(t *testing.T) {
	for file, into := range map[string]any{
		"lookup.json": &LookupResponse{}, "lookup-two-persons.json": &LookupResponse{}, "person.json": &PersonResponse{},
		"writers.json": &WritersResponse{}, "songwriters.json": &SongwritersResponse{}, "version.json": &VersionResponse{},
	} {
		raw, err := os.ReadFile(filepath.Join("testdata", "fixtures", file))
		if err != nil {
			t.Fatal(err)
		}
		// The client ignores unknown fields, so the API can add one; the fixtures were recorded from the same API
		// version as the types, so here every field must land somewhere.
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(into); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
	var answer LookupResponse
	raw, _ := os.ReadFile("testdata/fixtures/lookup.json")
	_ = json.Unmarshal(raw, &answer)
	if len(answer.Candidates) == 0 || answer.Candidates[0].PersistentID == "" || len(answer.Candidates[0].Records) == 0 {
		t.Errorf("a real lookup should decode into candidates with an ID and records: %+v", answer.Candidates)
	}
}

// Every exported name in the package has a doc comment that is a sentence: a type's or function's starts with its
// name, as godoc expects, and a field's ends with a full stop. The docs are the point of the types, so a missing one
// fails here rather than in review.
func TestEveryExportedNameIsDocumented(t *testing.T) {
	fset := token.NewFileSet()
	files, _ := filepath.Glob("*.go")
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		check := func(name string, doc *ast.CommentGroup, startsWithName bool) {
			text := strings.TrimSpace(doc.Text())
			switch {
			case doc == nil || text == "":
				t.Errorf("%s: %s has no doc comment", path, name)
			case startsWithName && !strings.HasPrefix(text, name+" ") && !strings.HasPrefix(text, "An "+name+" ") &&
				!strings.HasPrefix(text, "A "+name+" "):
				t.Errorf("%s: %s's comment should start with its name: %q", path, name, firstLine(text))
			case !strings.HasSuffix(firstParagraph(text), ".") && !strings.HasSuffix(firstParagraph(text), ":"):
				t.Errorf("%s: %s's comment should end its sentence: %q", path, name, firstParagraph(text))
			}
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() && (d.Recv == nil || exportedRecv(d.Recv)) {
					check(d.Name.Name, d.Doc, true)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if !s.Name.IsExported() {
							continue
						}
						doc := s.Doc
						if doc == nil {
							doc = d.Doc
						}
						check(s.Name.Name, doc, true)
						if st, ok := s.Type.(*ast.StructType); ok {
							for _, f := range st.Fields.List {
								for _, n := range f.Names {
									if n.IsExported() {
										check(s.Name.Name+"."+n.Name, f.Doc, false)
									}
								}
							}
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								doc := s.Doc
								if doc == nil {
									doc = d.Doc
								}
								check(n.Name, doc, true)
							}
						}
					}
				}
			}
		}
	}
}

func exportedRecv(fields *ast.FieldList) bool {
	expr := fields.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	ident, ok := expr.(*ast.Ident)
	return ok && unicode.IsUpper(rune(ident.Name[0]))
}

func firstLine(text string) string { line, _, _ := strings.Cut(text, "\n"); return line }

func firstParagraph(text string) string {
	para, _, _ := strings.Cut(text, "\n\n")
	return strings.TrimSpace(para)
}
