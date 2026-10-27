# songlinks-go

[![Go Reference](https://pkg.go.dev/badge/github.com/Alexkane/songlinks-go.svg)](https://pkg.go.dev/github.com/Alexkane/songlinks-go)
[![ci](https://github.com/Alexkane/songlinks-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Alexkane/songlinks-go/actions/workflows/ci.yml)
[![licence: MIT](https://img.shields.io/badge/licence-MIT-blue.svg)](LICENSE)

**Find the songwriter behind a name, an IPI or an ISRC.** The official Go SDK for [Songlinks](https://songlinks.io):
look up a songwriter by name or IPI name number, or find who wrote a recording from its ISRC, and get the person behind
every registration, with their works and ISWCs.

One songwriter is rarely one record. They're registered under different spellings, under several IPI name numbers, or
with no number at all. Songlinks groups those registrations into one person, shows the evidence for every join, and
keeps apart the ones it isn't sure of.

- **Free, with no key and no sign-up.**
- **Typed end to end.** Every answer is a Go struct, with a description of every field taken from the API's own
  description of itself.
- **No dependencies.** The standard library only, on Go 1.23 and later.
- **Built for real apps.** Clear errors, automatic retries when the API is busy, and long lists paged for you.

> **New to this?** Skip to [Your first program, step by step](#your-first-program-step-by-step): it starts from
> installing the language and ends with a working answer. The [words used here](#the-words-used-here) are explained too.

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
	if answer.Answer == "rejected" { // for example a one-word name, which matches too many people
		log.Fatalf("no answer: %s", answer.Rejection.Code)
	}
	candidate := answer.Candidates[0]
	fmt.Println(candidate.PersistentID, candidate.Names, candidate.Works)
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
> `Provenance.MLCSnapshot` gives its date, and `client.Version(ctx)` gives the current one). Some answers are wrong and some are incomplete. Not for rights or royalty
> decisions.

### More

```go
// Which of several writers with the same name wrote a song? Pass one of their songs.
answer, err := client.Lookup(ctx, "amy allen", &songlinks.LookupOptions{Song: "espresso"})
// answer.SongMatch.Match == "exact", answer.SongMatch.Candidates == [c1]: the candidate whose Ref is "c1" is the
// Amy Allen credited on that song

// Every work of a person, paged for you. Breaking out of the loop stops the requests.
for work, err := range client.PersonWorks(ctx, "SLP-ZYM7-GD3E-S", 0) {
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(work.Title)
}
```

### Who wrote this recording? Songwriters by ISRC

A released recording usually has an ISRC, a 12-character code such as `USMC19959123` (see
[the words used here](#the-words-used-here)). Give `Songwriters` up to 50 of them and it says, for each one, which songs the recording is registered to, who wrote
each song, and each writer's person ID:

```go
// A real ISRC written with dashes, a well-formed code no recording is registered under, and something that is not an
// ISRC at all.
answer, err := client.Songwriters(ctx, []string{"US-MC1-99-59123", "ZZZZZ0000000", "nope"})
if err != nil {
	log.Fatal(err)
}
for _, recording := range answer.Recordings {
	fmt.Println(recording.ISRC, recording.Status)
	for _, work := range recording.Works {
		for _, w := range work.Writers {
			fmt.Println("  ", work.Title, w.Name, w.IPINameNumber, w.Role, w.PersistentID)
		}
	}
}
// USMC19959123 registered
//    ALL THE SMALL THINGS MARK HOPPUS 00338980326 ComposerLyricist SLP-ZYM7-GD3E-S
//    ALL THE SMALL THINGS TRAVIS L BARKER 00340247397 ComposerLyricist SLP-MG23-XM9B-Y
//    ALL THE SMALL THINGS THOMAS DELONGE 00338976704 ComposerLyricist SLP-MEA6-3QQJ-C
// ZZZZZ0000000 unregistered
// nope invalid
```

Each recording's `Status` is one of three:

| Status | What it means | `Works` |
| --- | --- | --- |
| `registered` | The recording is registered to at least one song (work). | The songs, each with its writers. |
| `unregistered` | No song in the registration data lists this recording. It is an answer, not an error: the recording may not have been linked to its song yet, or the code may be mistyped. | Empty. |
| `invalid` | The entry is not an ISRC (two letters, three letters or digits, then seven digits). It does not fail the call. `ISRC` is your text as you sent it. | Empty. |

Things to know:

- **A writer's `PersistentID` can be empty, and often is.** It is empty when the writer's record isn't part of a stored
  person yet, for example because the writer was registered with no IPI name number. Nearly half of registered
  recordings have at least one writer like that. The writer's `Name`, `Role` and `IPINameNumber` (when there is one)
  are still the answer; `PersonURL` is empty too, and otherwise is a page such as `/person/SLP-ZYM7-GD3E-S` on
  `https://songlinks.io`.
- **Match answers to your list by ISRC, not by position.** Case, dashes and spaces are ignored, so `US-MC1-99-59123` and
  `usmc19959123` are one recording and come back as one answer, written `USMC19959123`. `songlinks.NormalizeISRC`
  turns your input into the same form.
- **Only the plain dash is ignored.** A word processor sometimes turns `-` into a longer dash (`–`); an ISRC written
  with one comes back `invalid`.
- **Writers only.** Publishers and administrators on the song are left out, and so are ownership shares.
- **One entry, one ISRC.** An entry holding a comma, semicolon or line break is refused, because the API would read it
  as several; more than 50 entries is refused too, so send a long list in batches of `songlinks.SongwritersMax`.

## Your first program, step by step

Never called an API from code before? This takes about five minutes.

**Before you start**, you need two things:

- **A terminal**, where you type commands. On a Mac, open Terminal (in Applications, then Utilities). On Windows, open
  PowerShell from the Start menu. Type each command below on its own line and press Enter after it.
- **A plain-text editor** for code, such as the free [Visual Studio Code](https://code.visualstudio.com). A word
  processor (Word, or TextEdit in its usual mode) adds formatting that stops code from running.
1. **Install Go** 1.23 or later from [go.dev/dl](https://go.dev/dl/). Check it worked by typing `go version` in a
   terminal; it prints a version such as `go version go1.23.4`.
2. **Make a folder for your project** (a "module", in Go's words) and add the SDK to it:

   ```sh
   mkdir songlinks-test
   cd songlinks-test
   go mod init songlinks-test
   go get github.com/Alexkane/songlinks-go
   ```

   `go get` prints the version it added, such as `go: added github.com/Alexkane/songlinks-go v0.1.0`.
3. **Create a file** called `main.go` in the `songlinks-test` folder, in your editor, with this in it:

   ```go
   package main

   import (
   	"context"
   	"fmt"
   	"log"
   	"os"
   	"strings"

   	"github.com/Alexkane/songlinks-go"
   )

   func main() {
   	name := "mark hoppus"
   	if len(os.Args) > 1 {
   		name = os.Args[1]
   	}
   	client := songlinks.New(songlinks.WithUserAgent("my-first-test/1.0"))
   	answer, err := client.Lookup(context.Background(), name, nil)
   	if err != nil {
   		log.Fatal(err)
   	}
   	if answer.Answer == "rejected" {
   		fmt.Println("No answer, because:", answer.Rejection.Code)
   		return
   	}
   	for _, person := range answer.Candidates {
   		id := person.PersistentID
   		if id == "" {
   			id = "(no person ID yet)"
   		}
   		fmt.Printf("%s %s (works: %d)\n", id, strings.Join(person.Names, ", "), person.Works)
   	}
   }
   ```

   `WithUserAgent` names your program to the API, so a problem can be traced to it; any short name will do.
4. **Run it**, from the same folder, with any songwriter's full name:

   ```sh
   go mod tidy
   go run . "mark hoppus"
   ```

   `go mod tidy` records what `main.go` uses; you only need it once. You should see
   `SLP-ZYM7-GD3E-S MARK HOPPUS (works: 302)`.

   **Keep the quotes** around the name: without them, only the first word is sent.

   **What you might see instead:**
   - **Several lines.** The name could belong to more than one person, so each possible person gets a line.
   - **`(no person ID yet)`.** Songlinks found the registrations but hasn't given that person an ID yet. That is
     normal, and the rest of the line is still the answer.
   - **`No answer, because: single-token`.** The name is one word, such as `madonna`, which matches too many people.
     Use the full name.
   - **`No answer, because: no-match`.** Nothing registered is close to that name. Check the spelling.

From here, the [examples](#examples) do real jobs: copy one from the
[examples folder on GitHub](https://github.com/Alexkane/songlinks-go/tree/main/examples) into your folder as
`main.go` and run it the same way. [Reading an answer](#reading-an-answer) explains what comes back, and the
[words used here](#the-words-used-here) are explained just below.

## The words used here

Music-publishing words, which you'll meet in the answers:

- **Songwriter registration**: when a song is written, its publisher registers it so royalties reach the people who
  wrote it. Each registration names the writers, as typed by whoever filed it. One songwriter can therefore appear
  under several spellings, and two different songwriters can share a name.
- **Record**: one of those registered writer entries: a name as it was typed, with its IPI name number if it has one,
  and the songs (works) registered under it.
- **IPI name number**: an 11-digit number that identifies a songwriter or publisher to collecting societies, such as
  `00338980326`. Many records have none, and one person can have several.
  [More about IPI name numbers](https://songlinks.io/guides/ipi-name-numbers).
- **Work**: a song as a composition (the melody and lyrics), as opposed to a particular recording of it.
- **ISWC**: the international code for a work, such as `T0719661581`. Not every registered work has one.
- **ISRC**: the international code for one recording, such as `USMC19959123` for blink-182's recording of "All the
  Small Things". An ISWC names the song (the composition); an ISRC names one recording of it, so a song that has been
  recorded many times has one ISWC and many ISRCs. You'll usually find ISRCs in a distributor's or label's catalogue
  export. They're written with or without dashes (`US-MC1-99-59123`); Songlinks reads both.
- **Person** and **person ID**: Songlinks groups the records it judges to be one human into a person and gives them a
  person ID, such as `SLP-ZYM7-GD3E-S`. Every person has a page, such as
  [songlinks.io/person/SLP-ZYM7-GD3E-S](https://songlinks.io/person/SLP-ZYM7-GD3E-S).
- **Candidate**: what a lookup by name returns: the records that name reaches, grouped into people for that one
  question. A candidate usually matches one person ID, but it is worked out fresh each time.
- **Evidence**: why two records were grouped: the same IPI name number, or a judgement that they are the same person
  (for example, the same co-writers and publishers).
- **Rejected**: a lookup that gave no candidates, with a reason, such as a one-word name (`madonna`) that matches too
  many people to group. It is an answer, not an error.

Programming words:

- **API**: a service that programs talk to over the internet. The Songlinks API answers questions such as "who is
  this songwriter?".
- **SDK**: this library. It does the talking to the API for you, so your code calls a function instead of building
  web requests by hand.
- **Rate limit**: the most requests the API accepts from one internet address in a stretch of time: about 100 every
  5 minutes, or 20 from VPNs and cloud servers. Over it, the SDK tells you how long to wait.
- **Module**: a Go project, with a `go.mod` file that lists the libraries it uses, such as this one.

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
| [`songwriters-by-isrc`](examples/songwriters-by-isrc/main.go) | A list of ISRCs in, CSV out: the songs each recording is registered to, their writers and person IDs. |

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
| `Songwriters(ctx, isrcs)` | `GET /v1/songwriters` | Up to 50 recordings checked at once, by ISRC: the songs each is registered to, their writers and the person behind each. |
| `Version(ctx)` | `GET /v1/version` | What is serving: the release, the model and the date of the registration data. |

The answer types are named after the API's schemas (`LookupResponse`, `PersonResponse`, `Cluster`, `WritersResponse`,
`SongwritersResponse` and the rest), with Go's initialisms (`PersistentID`, `IPINameNumber`, `ISWC`, `ISRC`). Two are
renamed: the API's `Record` is `LookupRecord` and its `Error` is `APIError`. The full reference is on
[pkg.go.dev](https://pkg.go.dev/github.com/Alexkane/songlinks-go).

### Reading an answer

A **person** and a **candidate** are different things: see [the words used here](#the-words-used-here).

- **A candidate can count more works than its person.** "mark hoppus" reaches two registrations, 302 works between
  them; the person `SLP-ZYM7-GD3E-S` holds one of them as theirs (294 works) and lists the other in `Possible`.
  `Person` is the authority on what an ID holds.
- **Match a writers check by number, not position.** Repeats and two forms of one number come back as one entry, as
  11 digits. `songlinks.NormalizeIPI` turns your input into the same form. A songwriters check is the same, by ISRC,
  with `songlinks.NormalizeISRC`.
- **A missing string is `""`.** Many records have no person ID yet, and that is ordinary. A number, boolean or object
  the API may leave out is a pointer, because there nil and zero say different things: a `WriterAnswer`'s `Works` is
  nil unless its `Status` is `"person"`, and a false `Prerelease` is a released ID.
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
// Continuing inside main above, where client and ctx were created (and with "errors" imported).
personID := "SLP-0000-0000-0" // an ID no one holds, to see the not-found branch
_, err := client.Person(ctx, personID, nil)

var withdrawn *songlinks.PersonWithdrawnError
var apiErr *songlinks.Error
switch {
case errors.As(err, &withdrawn):
	fmt.Println("withdrawn at", withdrawn.Person.WithdrawnAt) // a 410, and a valid answer to keep
case errors.As(err, &apiErr) && apiErr.Code == "rate_limited":
	fmt.Println("retry in", apiErr.RetryAfter)
case errors.As(err, &apiErr) && apiErr.Code == "not_found":
	fmt.Println("no person with that ID") // a 404, such as for SLP-0000-0000-0
case errors.As(err, &apiErr):
	fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.Message)
case err != nil:
	log.Fatal(err) // a network failure, after the client's retries
}
```

- **Retried automatically:** `429` and `503` answers and network failures, up to `WithMaxRetries` times (default 2).
  When the API asks for a wait (in a `Retry-After` header or the body's `retry_after_seconds`) of a minute or less,
  the client waits that long; otherwise it backs off from half a second.
- **Returned at once:** a requested wait longer than a minute, in `Error.RetryAfter`, for you to schedule.
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

The API is free and open. It allows about 100 requests per 5 minutes from one address, across every endpoint, and 20
from VPNs, proxies and hosting providers, which includes most cloud servers. A `Writers` or `Songwriters` call counts
as one request however many numbers or ISRCs it holds. Please send a `WithUserAgent` that names your app, keep to one
request at a time per user action, and cache answers you reuse. Over the limit, the SDK returns an `*Error` with
`Code` `"rate_limited"` whose `RetryAfter` is the most you need to wait; for a whole roster, see
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
