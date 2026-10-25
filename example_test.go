package songlinks_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/Alexkane/songlinks-go"
)

// These examples call the live API, so they are compiled but not run by `go test`; their output is shown as comments.

func ExampleClient_Lookup() {
	client := songlinks.New(songlinks.WithUserAgent("my-app/1.0"))
	answer, err := client.Lookup(context.Background(), "mark hoppus", nil)
	if err != nil {
		log.Fatal(err)
	}
	if answer.Answer == "rejected" {
		fmt.Println("no answer:", answer.Rejection.Code)
		return
	}
	for _, person := range answer.Candidates {
		fmt.Println(person.PersistentID, person.Names, person.Works)
	}
	// SLP-ZYM7-GD3E-S [MARK HOPPUS] 302
}

func ExampleClient_Lookup_song() {
	// Which of several writers with the same name wrote a song? Pass one of their songs.
	client := songlinks.New(songlinks.WithUserAgent("my-app/1.0"))
	answer, err := client.Lookup(context.Background(), "amy allen", &songlinks.LookupOptions{Song: "espresso"})
	if err != nil {
		log.Fatal(err)
	}
	if answer.SongMatch != nil {
		fmt.Println(answer.SongMatch.Match, answer.SongMatch.Candidates)
	}
	// exact [c1]
}

func ExampleClient_Person() {
	client := songlinks.New(songlinks.WithUserAgent("my-app/1.0"))
	person, err := client.Person(context.Background(), "SLP-ZYM7-GD3E-S", &songlinks.PersonOptions{WorksLimit: 3})
	if err != nil {
		log.Fatal(err)
	}
	for _, work := range person.WorksList.Items {
		fmt.Println(work.Title)
	}
	// I MISS YOU
	// ALL THE SMALL THINGS
	// DOWN
}

func ExampleClient_PersonWorks() {
	client := songlinks.New(songlinks.WithUserAgent("my-app/1.0"))
	for work, err := range client.PersonWorks(context.Background(), "SLP-ZYM7-GD3E-S", 0) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(work.MLCWorkID, work.Title, work.ISWC)
	}
}

func ExampleClient_Writers() {
	client := songlinks.New(songlinks.WithUserAgent("my-app/1.0"))
	check, err := client.Writers(context.Background(), []string{"00338980326", "00340247397"})
	if err != nil {
		log.Fatal(err)
	}
	for _, w := range check.Writers {
		fmt.Println(w.IPINameNumber, w.Status, w.PersonID, w.Name)
	}
	// 00338980326 person SLP-ZYM7-GD3E-S MARK HOPPUS
	// 00340247397 person SLP-MG23-XM9B-Y TRAVIS L BARKER
}

func ExampleError() {
	client := songlinks.New()
	_, err := client.Person(context.Background(), "SLP-0000-0000-0", nil)

	var withdrawn *songlinks.PersonWithdrawnError
	var apiErr *songlinks.Error
	switch {
	case errors.As(err, &withdrawn):
		fmt.Println("withdrawn at", withdrawn.Person.WithdrawnAt) // a 410, and a valid answer to keep
	case errors.As(err, &apiErr):
		fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.Message)
	case err != nil:
		log.Fatal(err) // a network failure, after the client's retries
	}
}
