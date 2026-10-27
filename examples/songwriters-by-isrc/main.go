// Command songwriters-by-isrc finds who wrote each recording in a list of ISRCs: the songs each recording is
// registered to, their writers, and each writer's person ID where Songlinks has one.
//
//	go run ./examples/songwriters-by-isrc isrcs.txt > songwriters.csv
//
// The input is one ISRC per line, such as USMC19959123 (dashes and spaces are fine). ISRCs are sent 50 at a time, the
// most the API takes in one call. The output has one row per writer of each song, so a recording with one song and
// three writers is three rows; a recording with no song, or a line that is not an ISRC, is one row with its status.
package main

import (
	"context"
	"encoding/csv"
	"log"
	"os"
	"strings"

	"github.com/Alexkane/songlinks-go"
)

func main() {
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	var isrcs []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			isrcs = append(isrcs, line)
		}
	}

	client := songlinks.New(songlinks.WithUserAgent("songlinks-example/1.0"))
	answers := map[string]songlinks.RecordingAnswer{}
	for start := 0; start < len(isrcs); start += songlinks.SongwritersMax {
		found, err := client.Songwriters(context.Background(), isrcs[start:min(start+songlinks.SongwritersMax, len(isrcs))])
		if err != nil {
			log.Fatal(err)
		}
		for _, r := range found.Recordings {
			answers[r.ISRC] = r
		}
	}

	out := csv.NewWriter(os.Stdout)
	defer out.Flush()
	_ = out.Write([]string{"isrc", "status", "song_code", "title", "iswc", "writer", "ipi", "role", "person_id"})
	for _, isrc := range isrcs {
		// The answer writes each ISRC as 12 characters, upper case, with repeats merged, so look yours up in the
		// same form.
		r := answers[songlinks.NormalizeISRC(isrc)]
		if len(r.Works) == 0 {
			_ = out.Write([]string{isrc, r.Status, "", "", "", "", "", "", ""})
			continue
		}
		for _, work := range r.Works {
			for _, w := range work.Writers {
				// An empty person_id is ordinary: the writer's record is not part of a stored person yet.
				_ = out.Write([]string{isrc, r.Status, work.SongCode, work.Title, work.ISWC, w.Name, w.IPINameNumber, w.Role,
					w.PersistentID})
			}
		}
	}
}
