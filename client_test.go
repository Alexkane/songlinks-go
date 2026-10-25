package songlinks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type reply struct {
	status  int
	headers map[string]string
	body    any
}

// server answers each request with the next scripted reply and records what was asked.
type server struct {
	t        *testing.T
	mu       sync.Mutex
	replies  []reply
	requests []*http.Request
	waits    []time.Duration
	*httptest.Server
}

func newServer(t *testing.T, replies ...reply) *server {
	s := &server{t: t, replies: replies}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.requests = append(s.requests, r)
		if len(s.replies) == 0 {
			t.Errorf("no scripted reply left for %s", r.URL)
			w.WriteHeader(500)
			return
		}
		next := s.replies[0]
		s.replies = s.replies[1:]
		for k, v := range next.headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(next.status)
		if next.body != nil {
			_ = json.NewEncoder(w).Encode(next.body)
		}
	}))
	t.Cleanup(s.Close)
	saved := sleep
	sleep = func(_ context.Context, d time.Duration) error { s.waits = append(s.waits, d); return nil }
	t.Cleanup(func() { sleep = saved })
	return s
}

func (s *server) client(options ...Option) *Client {
	return New(append([]Option{WithBaseURL(s.URL)}, options...)...)
}

func (s *server) query(i int) url.Values { return s.requests[i].URL.Query() }

var ctx = context.Background()

func TestLookupSendsTheNameAndOptions(t *testing.T) {
	s := newServer(t, reply{200, nil, map[string]any{"answer": "candidates"}})
	answer, err := s.client(WithUserAgent("my-app/1.0")).Lookup(ctx, "mark hoppus",
		&LookupOptions{Song: "down", Threshold: 0.8, Routes: []string{}})
	if err != nil || answer.Answer != "candidates" {
		t.Fatalf("got %+v, %v", answer, err)
	}
	r := s.requests[0]
	if r.URL.Path != "/v1/lookup" {
		t.Errorf("path %s", r.URL.Path)
	}
	want := url.Values{"name": {"mark hoppus"}, "song": {"down"}, "threshold": {"0.8"}, "routes": {"none"}}
	if r.URL.Query().Encode() != want.Encode() {
		t.Errorf("query %s, want %s", r.URL.Query().Encode(), want.Encode())
	}
	if ua := r.Header.Get("User-Agent"); ua != "songlinks-go/"+Version+" my-app/1.0" {
		t.Errorf("User-Agent %q", ua)
	}
}

func TestLookupLeavesUnsetOptionsToTheAPI(t *testing.T) {
	s := newServer(t, reply{200, nil, map[string]any{}})
	if _, err := s.client().Lookup(ctx, "amy allen", nil); err != nil {
		t.Fatal(err)
	}
	if q := s.query(0).Encode(); q != "name=amy+allen" {
		t.Errorf("query %s", q)
	}
}

func TestPersonEscapesTheIDAndPages(t *testing.T) {
	s := newServer(t, reply{200, nil, map[string]any{"person_id": "SLP-1"}})
	if _, err := s.client().Person(ctx, "SLP/../x", &PersonOptions{WorksOffset: 200, WorksLimit: 50}); err != nil {
		t.Fatal(err)
	}
	if p := s.requests[0].URL.EscapedPath(); p != "/v1/person/SLP%2F..%2Fx" {
		t.Errorf("path %s", p)
	}
	if q := s.query(0).Encode(); q != "works_limit=50&works_offset=200" {
		t.Errorf("query %s", q)
	}
}

func TestAMergedIDIsFollowed(t *testing.T) {
	s := newServer(t,
		reply{308, map[string]string{"Location": "/v1/person/SLP-NEW"}, nil},
		reply{200, nil, map[string]any{"person_id": "SLP-NEW", "redirected_from": "SLP-OLD"}})
	p, err := s.client().Person(ctx, "SLP-OLD", nil)
	if err != nil || p.RedirectedFrom != "SLP-OLD" {
		t.Fatalf("got %+v, %v", p, err)
	}
}

func page(titles []string, next any) reply {
	items := []map[string]any{}
	for _, title := range titles {
		items = append(items, map[string]any{"title": title})
	}
	return reply{200, nil, map[string]any{"works_list": map[string]any{"items": items, "next_offset": next}}}
}

func TestPersonWorksReadsEveryPageAndStopsWhenAsked(t *testing.T) {
	s := newServer(t, page([]string{"A", "B"}, 2), page([]string{"C"}, nil))
	var titles []string
	for work, err := range s.client().PersonWorks(ctx, "SLP-1", 2) {
		if err != nil {
			t.Fatal(err)
		}
		titles = append(titles, work.Title)
	}
	if strings.Join(titles, ",") != "A,B,C" {
		t.Errorf("titles %v", titles)
	}
	if s.query(1).Get("works_offset") != "2" {
		t.Errorf("second page asked for offset %s", s.query(1).Get("works_offset"))
	}

	s = newServer(t, page([]string{"A", "B"}, 2))
	for range s.client().PersonWorks(ctx, "SLP-1", 0) {
		break
	}
	if len(s.requests) != 1 || s.query(0).Get("works_limit") != "200" {
		t.Errorf("a page is fetched only when the loop needs it, at the most per page: %d requests", len(s.requests))
	}
}

func TestPersonWorksYieldsAnErrorOnce(t *testing.T) {
	s := newServer(t, reply{404, nil, map[string]any{"error": "not_found", "message": "No person with that ID."}})
	n := 0
	for _, err := range s.client().PersonWorks(ctx, "SLP-0", 0) {
		n++
		if err == nil {
			t.Error("want the error")
		}
	}
	if n != 1 {
		t.Errorf("yielded %d times", n)
	}
}

func TestWritersJoinsTheListAndRefusesABadOne(t *testing.T) {
	s := newServer(t, reply{200, nil, map[string]any{"writers": []any{}}})
	if _, err := s.client().Writers(ctx, []string{"338980326", "00340247397"}); err != nil {
		t.Fatal(err)
	}
	if q := s.query(0).Get("ipi"); q != "338980326,00340247397" {
		t.Errorf("ipi %s", q)
	}
	if _, err := s.client().Writers(ctx, nil); err == nil {
		t.Error("an empty list should be refused")
	}
	if _, err := s.client().Writers(ctx, make([]string, WritersMax+1)); err == nil {
		t.Error("a list over the limit should be refused")
	}
	if len(s.requests) != 1 {
		t.Error("a refused list must not reach the API")
	}
}

func TestAnErrorCarriesTheAPIsCode(t *testing.T) {
	s := newServer(t, reply{404, nil, map[string]any{"error": "not_found", "message": "No person with that ID."}})
	_, err := s.client().Person(ctx, "SLP-0", nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 || apiErr.Code != "not_found" ||
		apiErr.Message != "No person with that ID." {
		t.Fatalf("got %#v", err)
	}
}

func TestAValidationErrorIsInvalidRequest(t *testing.T) {
	s := newServer(t, reply{422, nil, map[string]any{"detail": []any{map[string]any{"msg": "bad"}}}})
	_, err := s.client().Lookup(ctx, "x", &LookupOptions{Threshold: 2})
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != "invalid_request" || apiErr.Message != "HTTP 422" {
		t.Fatalf("got %#v", err)
	}
}

func TestAWithdrawnPersonReturnsTheAPIsAnswer(t *testing.T) {
	s := newServer(t, reply{410, nil, map[string]any{"person_id": "SLP-W", "state": "withdrawn", "history": []any{}}})
	_, err := s.client().Person(ctx, "SLP-W", nil)
	var withdrawn *PersonWithdrawnError
	if !errors.As(err, &withdrawn) || withdrawn.Person.PersonID != "SLP-W" {
		t.Fatalf("got %#v", err)
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != "withdrawn" || apiErr.StatusCode != 410 {
		t.Errorf("the withdrawn error should also be an *Error: %#v", apiErr)
	}
}

func TestA503IsRetriedThenSucceeds(t *testing.T) {
	s := newServer(t, reply{503, nil, map[string]any{"error": "busy", "message": "Busy."}},
		reply{200, nil, map[string]any{"version": "0.11.0"}})
	v, err := s.client().Version(ctx)
	if err != nil || v.Version != "0.11.0" {
		t.Fatalf("got %+v, %v", v, err)
	}
	if len(s.waits) != 1 || s.waits[0] < 375*time.Millisecond || s.waits[0] > 625*time.Millisecond {
		t.Errorf("waits %v", s.waits)
	}
}

func TestA429WaitsRetryAfterAndFailsWhenRetriesRunOut(t *testing.T) {
	limited := reply{429, map[string]string{"Retry-After": "7"}, map[string]any{"error": "rate_limited", "message": "Slow down."}}
	s := newServer(t, limited, limited)
	_, err := s.client(WithMaxRetries(1)).Version(ctx)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != "rate_limited" || apiErr.RetryAfter != 7*time.Second {
		t.Fatalf("got %#v", err)
	}
	if len(s.waits) != 1 || s.waits[0] != 7*time.Second {
		t.Errorf("waits %v", s.waits)
	}
}

func TestRetryAfterIsCappedAtAMinute(t *testing.T) {
	s := newServer(t, reply{429, map[string]string{"Retry-After": "3600"}, nil}, reply{200, nil, map[string]any{}})
	if _, err := s.client().Version(ctx); err != nil {
		t.Fatal(err)
	}
	if len(s.waits) != 1 || s.waits[0] != time.Minute {
		t.Errorf("waits %v", s.waits)
	}
}

func TestOtherErrorsAreNotRetried(t *testing.T) {
	s := newServer(t, reply{400, nil, map[string]any{"error": "bad_request", "message": "No."}})
	if _, err := s.client().Version(ctx); err == nil || len(s.requests) != 1 {
		t.Errorf("err %v after %d requests", err, len(s.requests))
	}
}

func TestANetworkFailureIsRetriedThenReturned(t *testing.T) {
	s := newServer(t)
	s.Close() // nothing listens any more, so every attempt is refused
	if _, err := s.client().Version(ctx); err == nil || len(s.waits) != 2 {
		t.Errorf("err %v after %d waits", err, len(s.waits))
	}
}

func TestACancelledContextIsNotRetried(t *testing.T) {
	s := newServer(t)
	s.Close()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.client().Version(cancelled); !errors.Is(err, context.Canceled) || len(s.waits) != 0 {
		t.Errorf("err %v after %d waits", err, len(s.waits))
	}
}
