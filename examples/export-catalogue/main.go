// Command export-catalogue writes every work of a person as CSV.
//
//	go run ./examples/export-catalogue SLP-ZYM7-GD3E-S > works.csv
package main

import (
	"context"
	"encoding/csv"
	"log"
	"os"

	"github.com/Alexkane/songlinks-go"
)

func main() {
	personID := "SLP-ZYM7-GD3E-S"
	if len(os.Args) > 1 {
		personID = os.Args[1]
	}
	client := songlinks.New(songlinks.WithUserAgent("songlinks-example/1.0"))
	out := csv.NewWriter(os.Stdout)
	defer out.Flush()
	_ = out.Write([]string{"mlc_work_id", "title", "iswc"})
	// PersonWorks pages through the API 200 works at a time; nothing here needs to know about pages.
	for work, err := range client.PersonWorks(context.Background(), personID, 0) {
		if err != nil {
			log.Fatal(err)
		}
		_ = out.Write([]string{work.MLCWorkID, work.Title, work.ISWC})
	}
}
