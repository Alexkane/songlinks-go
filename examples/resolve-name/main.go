// Command resolve-name prints each person a songwriter's name could be, with their registrations and what joined them.
//
//	go run ./examples/resolve-name "mark hoppus"
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/Alexkane/songlinks-go"
)

func main() {
	name := "mark hoppus"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	client := songlinks.New(songlinks.WithUserAgent("songlinks-example/1.0"))
	answer, err := client.Lookup(context.Background(), name, nil)
	if err != nil {
		log.Fatal(err)
	}
	if answer.Answer == "rejected" {
		// Not an error: the API explains why it gave no candidates, such as a name too common to group.
		fmt.Println("No answer:", answer.Rejection.Code)
		// Nearest suggests registered names close to the one asked, when there are any.
		if len(answer.Rejection.Nearest) > 0 {
			fmt.Println("Did you mean:", strings.Join(answer.Rejection.Nearest, ", "))
		}
		return
	}
	for _, person := range answer.Candidates {
		// PersistentID is empty when no ID has been issued yet, which is common; the records are still the answer.
		id := person.PersistentID
		if id == "" {
			id = "(no person ID yet)"
		}
		fmt.Printf("%s  %s  %d works\n", id, strings.Join(person.Names, ", "), person.Works)
		for _, r := range person.Records {
			ipi := r.IPINameNumber
			if ipi == "" {
				ipi = "-"
			}
			fmt.Printf("    %-30s IPI %-12s %d works\n", r.Name, ipi, r.Works)
		}
		var rules []string
		for _, e := range person.Evidence {
			if !slices.Contains(rules, e.Rule) {
				rules = append(rules, e.Rule)
			}
		}
		slices.Sort(rules)
		if len(rules) == 0 {
			rules = []string{"nothing to join: one record"}
		}
		fmt.Println("    joined by:", strings.Join(rules, ", "))
	}
	// Pairs the model would not join are shown apart, never as part of a person.
	if len(answer.Review) > 0 {
		fmt.Printf("\n%d pair(s) held apart for review.\n", len(answer.Review))
	}
}
