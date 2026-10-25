package songlinks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Version is this SDK's release.
const Version = "0.1.1"

// DefaultBaseURL is the API's address.
const DefaultBaseURL = "https://songlinks.io"

// WritersMax is the most IPI name numbers [Client.Writers] accepts in one call, the API's own limit.
const WritersMax = 50

// WorksPageMax is the most works one [Client.Person] call returns; [Client.PersonWorks] pages through the rest.
const WorksPageMax = 200

// maxWait is the longest wait the client sleeps through before retrying; a longer one is returned to the caller at
// once.
const maxWait = time.Minute

// NormalizeIPI writes an IPI name number the way [Client.Writers] answers do, so the answers can be matched to the
// numbers you sent. Spaces and dashes (including the typographic dashes a spreadsheet substitutes) are dropped, and a
// run of 1 to 11 digits is padded with leading zeros to 11: "338980326" and "003-389-803 26" both become
// "00338980326". Anything else is not an IPI name number, and is returned with surrounding spaces removed and cut to
// 32 characters, as the API echoes it with Status "invalid". A dot is not a separator: "15036818.0" is what a
// spreadsheet makes of a number read as a float, and dropping the dot would turn it into somebody else's number.
func NormalizeIPI(ipiNameNumber string) string {
	digits := strings.Map(func(r rune) rune {
		if isIPISeparator(r) {
			return -1
		}
		return r
	}, ipiNameNumber)
	if ipiDigits.MatchString(digits) {
		return strings.Repeat("0", 11-len(digits)) + digits
	}
	// Characters, not bytes, as the API counts them: a byte cut could split a character in two.
	trimmed := []rune(strings.TrimFunc(ipiNameNumber, isPythonSpace))
	if len(trimmed) > 32 {
		trimmed = trimmed[:32]
	}
	return string(trimmed)
}

// ipiDigits mirrors the API's own reading of an IPI name number (writer/service.py ipi_key): 1 to 11 ASCII digits.
var ipiDigits = regexp.MustCompile(`^[0-9]{1,11}$`)

// isIPISeparator is the API's separator class, `[\s\-\u2010-\u2015\u2212]` in Python, where \s is any Unicode
// space: a number pasted from a web page often carries non-breaking spaces, which Go's regexp \s (ASCII only) keeps.
func isIPISeparator(r rune) bool {
	return isPythonSpace(r) || r == '-' || (r >= 0x2010 && r <= 0x2015) || r == 0x2212
}

// isPythonSpace is what Python's \s accepts: Unicode spaces plus the ASCII separators 0x1c-0x1f.
func isPythonSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// Client calls the Songlinks API. Create one with [New] and reuse it; it is safe for concurrent use.
type Client struct {
	baseURL    string
	httpClient *http.Client
	maxRetries int
	userAgent  string
}

// An Option configures a [Client]. Pass options to [New].
type Option func(*Client)

// WithBaseURL sets the API's address. The default is [DefaultBaseURL].
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithHTTPClient sets the [http.Client] requests are sent with, for a proxy, custom TLS or tracing. The default has a
// 30-second timeout. A merged person ID answers with a redirect, so a client that refuses redirects breaks
// [Client.Person] for merged IDs.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) { c.httpClient = httpClient }
}

// WithMaxRetries sets how many times a request is retried after a 429, a 503 or a network failure. The default is 2;
// 0 turns retrying off.
func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

// WithUserAgent adds your app's name and version, such as "my-app/1.2", to each request's User-Agent after the SDK's
// own.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) { c.userAgent = "songlinks-go/" + Version + " " + userAgent }
}

// New creates a client. With no options it calls https://songlinks.io with a 30-second timeout and two retries.
func New(options ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		maxRetries: 2,
		userAgent:  "songlinks-go/" + Version,
	}
	for _, option := range options {
		option(c)
	}
	return c
}

// LookupOptions are the optional parts of a lookup. A zero field is left to the API.
type LookupOptions struct {
	// Song is the title of a song the writer wrote. When given, the answer's SongMatch says which candidates hold a
	// work with that title.
	Song string
	// Threshold is how sure Songlinks must be before it joins two records into one person, from 0.5 to 0.99. Higher
	// joins fewer records and keeps more apart. Zero leaves the API's default, which [Client.Version] reports as
	// Threshold (0.85 today).
	Threshold float64
	// Routes are extra searches to run beside the name match. The one there is, "catalogue", also finds records
	// registered under another name that share the writer's catalogue, such as a stage name, and returns them in
	// Links. Nil runs it, as the API does by default; an empty, non-nil slice skips it, and the lookup is faster.
	Routes []string
}

// PersonOptions choose which page of a person's works [Client.Person] returns. A zero field is left to the API.
type PersonOptions struct {
	// WorksOffset is the position of the first work to return, from 0.
	WorksOffset int
	// WorksLimit is how many works to return, from 1 to 200. Zero leaves the API's default of 100.
	WorksLimit int
}

// Lookup looks up a songwriter by name, such as "mark hoppus", and returns the people the name could be, each with the
// registrations behind them. Check the answer's Answer field first: "rejected" is an answer, not an error, and
// Rejection says why. A single word, such as "madonna", is rejected with Rejection.Code "single-token", since one word
// matches too many people to group. Options may be nil.
//
// It returns an error, without calling the API, when the name is empty, and an [*Error] when the API answers with an
// error, such as Code "invalid_request" for a Threshold out of range.
func (c *Client) Lookup(ctx context.Context, name string, options *LookupOptions) (*LookupResponse, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("songlinks: Lookup needs a name")
	}
	query := url.Values{"name": {name}}
	if options != nil {
		if options.Song != "" {
			query.Set("song", options.Song)
		}
		if options.Threshold != 0 {
			query.Set("threshold", strconv.FormatFloat(options.Threshold, 'f', -1, 64))
		}
		if options.Routes != nil {
			// The API reads an empty `routes` as "use the default", so no routes at all is spelt `none`.
			routes := strings.Join(options.Routes, ",")
			if routes == "" {
				routes = "none"
			}
			query.Set("routes", routes)
		}
	}
	var answer LookupResponse
	return &answer, c.get(ctx, "/v1/lookup", query, &answer)
}

// Person reads a stored person by ID, such as "SLP-ZYM7-GD3E-S", with one page of their works. A merged ID is
// followed to the person it now points at, and the answer's RedirectedFrom names the ID you asked for. Options may be
// nil, for the first page.
//
// It returns an error, without calling the API, when the ID is empty; a [*PersonWithdrawnError] when the ID has been
// withdrawn; and an [*Error] for any other error answer, such as Code "not_found" for an unknown ID.
func (c *Client) Person(ctx context.Context, personID string, options *PersonOptions) (*PersonResponse, error) {
	// An empty ID would ask for /v1/person/, which is a different route and answers a bare 404.
	if strings.TrimSpace(personID) == "" {
		return nil, errors.New("songlinks: Person needs a person ID")
	}
	query := url.Values{}
	if options != nil {
		if options.WorksOffset != 0 {
			query.Set("works_offset", strconv.Itoa(options.WorksOffset))
		}
		if options.WorksLimit != 0 {
			query.Set("works_limit", strconv.Itoa(options.WorksLimit))
		}
	}
	var person PersonResponse
	return &person, c.get(ctx, "/v1/person/"+url.PathEscape(personID), query, &person)
}

// PersonWorks reads every work of a person, in the API's order, pageSize works per request. A pageSize of 0 or less,
// or above [WorksPageMax], means [WorksPageMax]. Pages are requested only as the loop asks for more, so breaking out
// of the loop stops the requests. An error ends the sequence: it is yielded once, with a zero PersonWork, and is the
// same error [Client.Person] returns, so a withdrawn ID yields a [*PersonWithdrawnError].
//
//	for work, err := range client.PersonWorks(ctx, "SLP-ZYM7-GD3E-S", 0) {
//		if err != nil {
//			return err
//		}
//		fmt.Println(work.Title)
//	}
func (c *Client) PersonWorks(ctx context.Context, personID string, pageSize int) iter.Seq2[PersonWork, error] {
	if pageSize <= 0 || pageSize > WorksPageMax {
		pageSize = WorksPageMax
	}
	return func(yield func(PersonWork, error) bool) {
		offset := 0
		for {
			page, err := c.Person(ctx, personID, &PersonOptions{WorksOffset: offset, WorksLimit: pageSize})
			if err != nil {
				yield(PersonWork{}, err)
				return
			}
			for _, work := range page.WorksList.Items {
				if !yield(work, nil) {
					return
				}
			}
			if page.WorksList.NextOffset == nil {
				return
			}
			offset = *page.WorksList.NextOffset
		}
	}
}

// Writers checks up to [WritersMax] IPI name numbers at once and returns the person who holds each and a status, with
// totals in Summary. Leading zeros may be left off, and spaces and dashes are ignored. The API refuses a longer list
// rather than shortening it, and so does this method, counting entries: for more, call it once per batch. Each entry
// is one number: an entry holding a comma, semicolon, line break or the text "%2C" is refused, since the API would
// read it as several.
//
// The answer has one entry per distinct number, in the order each was first sent, written as 11 digits. Two forms of
// one number ("338980326" and "00338980326") and repeats come back as one entry, so match answers to your list by
// number, not by position: [NormalizeIPI] turns your input into the form the answer uses.
//
// It returns an error, without calling the API, when the list is empty or too long, and an [*Error] when the API
// answers with an error.
func (c *Client) Writers(ctx context.Context, ipiNameNumbers []string) (*WritersResponse, error) {
	if len(ipiNameNumbers) == 0 {
		return nil, errors.New("songlinks: Writers needs at least one IPI name number")
	}
	if len(ipiNameNumbers) > WritersMax {
		return nil, fmt.Errorf("songlinks: Writers takes at most %d IPI name numbers per call, got %d", WritersMax, len(ipiNameNumbers))
	}
	for _, n := range ipiNameNumbers {
		// The API splits its list on these, and on the text %2C, so one entry holding one would be read as several.
		// The edges are trimmed first, as the API trims them, so a line read from a Windows file (ending "\r") passes.
		inner := strings.TrimFunc(n, isPythonSpace)
		if strings.ContainsAny(inner, ",;\r\n") || strings.Contains(strings.ToLower(inner), "%2c") {
			return nil, fmt.Errorf("songlinks: Writers takes one IPI name number per entry, got %q", n)
		}
	}
	var answer WritersResponse
	return &answer, c.get(ctx, "/v1/writers", url.Values{"ipi": {strings.Join(ipiNameNumbers, ",")}}, &answer)
}

// Version asks what is serving: the API's release, the model and code in use, and the date of the registration data.
//
// It returns an [*Error] when the API answers with an error.
func (c *Client) Version(ctx context.Context) (*VersionResponse, error) {
	var answer VersionResponse
	return &answer, c.get(ctx, "/v1/version", nil, &answer)
}

func (c *Client) get(ctx context.Context, path string, query url.Values, into any) error {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)

		// net/http follows the 307/308 a merged person ID answers with, re-sending the same GET.
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// A cancelled or expired context is the caller's decision, never retried.
			if ctx.Err() != nil || attempt >= c.maxRetries {
				return err
			}
			if err := sleep(ctx, backoff(attempt, 0)); err != nil {
				return err
			}
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode < 300 {
			return json.Unmarshal(body, into)
		}
		retryAfter := retryAfterOf(resp.Header.Get("Retry-After"), body)
		// A wait longer than a minute (the edge's rate limit asks for up to five) is the caller's to schedule, not
		// something to block on inside one call, so it is returned at once.
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable) &&
			attempt < c.maxRetries && retryAfter <= maxWait {
			if err := sleep(ctx, backoff(attempt, retryAfter)); err != nil {
				return err
			}
			continue
		}
		return toError(resp.StatusCode, body, retryAfter)
	}
}

// Error is an error answer from the API. Branch on Code, the API's own error code; Message is a sentence meant for
// people, and its wording may change.
//
//	var apiErr *songlinks.Error
//	if errors.As(err, &apiErr) && apiErr.Code == "not_found" {
//		...
//	}
type Error struct {
	// StatusCode is the HTTP status, such as 404 or 503.
	StatusCode int
	// Code is the API's error code, such as "not_found", "busy", "too_many" or "rate_limited". A 422, the API's
	// answer to a malformed parameter, is "invalid_request".
	Code string
	// Message is the API's message, or a description of the failure when it sent none.
	Message string
	// RetryAfter is how long the API asked the caller to wait before trying again, on a 429 or 503; zero when it
	// did not say. The public API allows about 100 requests per 5 minutes from one address, across every endpoint
	// (20 from VPNs, proxies and hosting providers, which includes most cloud servers).
	// The client waits out a wait of a minute or less and retries by itself, so an error with a short RetryAfter
	// means its retries ran out; a longer wait is returned at once. It is the most you need to wait, not a countdown.
	RetryAfter time.Duration
	// Body is the answer's body, as sent. Decode it into [APIError] for an ordinary error or [RateLimited] for a 429.
	Body json.RawMessage
}

// Error returns the status, the code and the API's message.
func (e *Error) Error() string {
	return fmt.Sprintf("songlinks: %d %s: %s", e.StatusCode, e.Code, e.Message)
}

// PersonWithdrawnError is returned by [Client.Person] when a person ID has been withdrawn (HTTP 410). The API still
// answers in full: Person says when the ID was issued and withdrawn, and gives its history. It is a valid answer to
// keep. errors.As also finds the [*Error] it wraps, with Code "withdrawn".
type PersonWithdrawnError struct {
	// Person is the API's answer about the withdrawn ID.
	Person PersonWithdrawnResponse
	err    *Error
}

// Error says which person ID was withdrawn.
func (e *PersonWithdrawnError) Error() string {
	return "songlinks: person " + e.Person.PersonID + " was withdrawn"
}

// Unwrap returns the [*Error] for the 410 answer, so code that handles every API error in one place still sees it.
func (e *PersonWithdrawnError) Unwrap() error { return e.err }

func toError(status int, body []byte, retryAfter time.Duration) error {
	var fields struct {
		Error   any    `json:"error"`
		Message any    `json:"message"`
		Person  string `json:"person_id"`
	}
	_ = json.Unmarshal(body, &fields) // A body that is not JSON leaves the fields empty, and the status speaks.
	e := &Error{StatusCode: status, RetryAfter: retryAfter, Body: body, Message: fmt.Sprintf("HTTP %d", status)}
	if m, ok := fields.Message.(string); ok {
		e.Message = m
	}
	// 422 is FastAPI's validation answer: no `error` or `message`, but a `detail` list saying which parameter was
	// wrong and why. Its first entry becomes the message, so a Threshold of 2 reads "threshold: Input should be less
	// than or equal to 0.99" rather than "HTTP 422".
	if status == http.StatusUnprocessableEntity {
		var invalid struct {
			Detail []struct {
				Loc []any  `json:"loc"`
				Msg string `json:"msg"`
			} `json:"detail"`
		}
		if json.Unmarshal(body, &invalid) == nil && len(invalid.Detail) > 0 && invalid.Detail[0].Msg != "" {
			d := invalid.Detail[0]
			e.Message = d.Msg
			if len(d.Loc) > 0 {
				e.Message = fmt.Sprint(d.Loc[len(d.Loc)-1]) + ": " + d.Msg
			}
		}
	}
	switch code, ok := fields.Error.(string); {
	case ok:
		e.Code = code
	case status == http.StatusTooManyRequests:
		e.Code = "rate_limited"
	case status == http.StatusUnprocessableEntity:
		// FastAPI's validation answer, which has `detail` rather than `error`.
		e.Code = "invalid_request"
	case status == http.StatusGone && fields.Person != "":
		e.Code = "withdrawn"
	default:
		e.Code = "http_" + strconv.Itoa(status)
	}
	if status == http.StatusGone && fields.Person != "" {
		withdrawn := &PersonWithdrawnError{err: e}
		if err := json.Unmarshal(body, &withdrawn.Person); err == nil {
			return withdrawn
		}
	}
	return e
}

// retryAfterOf is the wait the API asked for: the Retry-After header, or else the body's retry_after_seconds. The
// edge's rate limit answers with the body only (its 429 carries no header), so reading the header alone would retry a
// five-minute limit after half a second. Only the header's delta-seconds form is read: the API never sends an HTTP
// date. Zero means the API did not say.
func retryAfterOf(header string, body []byte) time.Duration {
	if seconds, err := strconv.ParseFloat(header, 64); err == nil && seconds >= 0 {
		return secondsToDuration(seconds)
	}
	var fields struct {
		RetryAfterSeconds *float64 `json:"retry_after_seconds"`
	}
	if json.Unmarshal(body, &fields) == nil && fields.RetryAfterSeconds != nil && *fields.RetryAfterSeconds >= 0 {
		return secondsToDuration(*fields.RetryAfterSeconds)
	}
	return 0
}

// secondsToDuration clamps before converting: a Duration holds about 292 years, and converting a larger float (or
// +Inf) is implementation-defined in Go, which could make an absurd wait look short and be retried at once.
func secondsToDuration(seconds float64) time.Duration {
	if seconds > 24*60*60 {
		return 24 * time.Hour
	}
	return time.Duration(seconds * float64(time.Second))
}

// backoff is the API's requested wait when it gives one; otherwise 0.5 s, 1 s, 2 s ... with jitter.
func backoff(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	base := 500 * time.Millisecond << attempt
	return time.Duration(float64(base) * (0.75 + rand.Float64()/2))
}

// sleep is a variable so tests can record waits instead of taking them.
var sleep = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
