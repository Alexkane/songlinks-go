// Command which-namesake answers: two writers share a name, which one wrote this song?
//
//	go run ./examples/which-namesake "amy allen" "espresso"
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
	name, song := "amy allen", "espresso"
	if len(os.Args) > 2 {
		name, song = os.Args[1], os.Args[2]
	}
	client := songlinks.New(songlinks.WithUserAgent("songlinks-example/1.0"))
	answer, err := client.Lookup(context.Background(), name, &songlinks.LookupOptions{Song: song})
	if err != nil {
		log.Fatal(err)
	}
	match := answer.SongMatch
	if match == nil || len(match.Candidates) == 0 {
		fmt.Printf("No one called %q in this answer has a work titled %q.\n", name, song)
		return
	}
	// SongMatch names candidates by their Ref (c1, c2 ...), which is only meaningful inside this one answer.
	byRef := map[string]songlinks.Cluster{}
	for _, c := range answer.Candidates {
		byRef[c.Ref] = c
	}
	fmt.Printf("%q: %s title match\n", song, match.Match)
	for _, ref := range match.Candidates {
		p := byRef[ref]
		id := p.PersistentID
		if id == "" {
			id = ref
		}
		fmt.Printf("  %s  %s  IPI %s\n", id, strings.Join(p.Names, ", "), strings.Join(p.IPINameNumbers, ", "))
	}
}
