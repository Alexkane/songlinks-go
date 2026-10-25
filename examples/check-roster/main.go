// Command check-roster checks a roster of IPI name numbers: who holds each, and how many works they have.
//
//	go run ./examples/check-roster roster.txt > checked.csv
//
// The input is one IPI name number per line. Numbers are sent 50 at a time, the most the API takes in one call.
package main

import (
	"context"
	"encoding/csv"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/Alexkane/songlinks-go"
)

func main() {
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	var ipis []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			ipis = append(ipis, line)
		}
	}

	client := songlinks.New(songlinks.WithUserAgent("songlinks-example/1.0"))
	answers := map[string]songlinks.WriterAnswer{}
	for start := 0; start < len(ipis); start += songlinks.WritersMax {
		check, err := client.Writers(context.Background(), ipis[start:min(start+songlinks.WritersMax, len(ipis))])
		if err != nil {
			log.Fatal(err)
		}
		for _, w := range check.Writers {
			answers[w.IPINameNumber] = w
		}
	}

	out := csv.NewWriter(os.Stdout)
	defer out.Flush()
	_ = out.Write([]string{"ipi", "status", "person_id", "name", "works"})
	for _, ipi := range ipis {
		// The answer writes each number as 11 digits, with repeats merged, so look yours up in the same form.
		w, ok := answers[songlinks.NormalizeIPI(ipi)]
		if !ok {
			_ = out.Write([]string{ipi, "", "", "", ""})
			continue
		}
		works := ""
		if w.Works != nil {
			works = strconv.Itoa(*w.Works)
		}
		_ = out.Write([]string{ipi, w.Status, w.PersonID, w.Name, works})
	}
}
