package songlinks

import (
	"os"
	"testing"
)

// The same calls against songlinks.io. Run with SONGLINKS_LIVE=1; skipped otherwise, so CI never depends on it.

const hoppus = "SLP-ZYM7-GD3E-S"

func live(t *testing.T) *Client {
	if os.Getenv("SONGLINKS_LIVE") != "1" {
		t.Skip("set SONGLINKS_LIVE=1")
	}
	return New(WithUserAgent("songlinks-go-tests"))
}

func TestLiveLookup(t *testing.T) {
	answer, err := live(t).Lookup(ctx, "mark hoppus", nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range answer.Candidates {
		found = found || c.PersistentID == hoppus
	}
	if answer.Answer != "candidates" || !found {
		t.Errorf("answer %q, candidates %+v", answer.Answer, answer.Candidates)
	}
}

func TestLivePersonAndEveryWork(t *testing.T) {
	c := live(t)
	person, err := c.Person(ctx, hoppus, &PersonOptions{WorksLimit: 3})
	if err != nil || len(person.WorksList.Items) != 3 {
		t.Fatalf("%+v, %v", person, err)
	}
	n := 0
	for _, err := range c.PersonWorks(ctx, hoppus, 100) {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	if n != person.WorksList.Total {
		t.Errorf("read %d works, the person has %d", n, person.WorksList.Total)
	}
}

func TestLiveWritersAndVersion(t *testing.T) {
	c := live(t)
	check, err := c.Writers(ctx, []string{"00338980326"})
	if err != nil || check.Writers[0].PersonID != hoppus {
		t.Fatalf("%+v, %v", check, err)
	}
	v, err := c.Version(ctx)
	if err != nil || v.Version == "" {
		t.Fatalf("%+v, %v", v, err)
	}
}
