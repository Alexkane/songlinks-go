// Package songlinks is the official Go SDK for the Songlinks API: find the songwriter behind a name.
//
// One songwriter is rarely one record. They are registered under different spellings, under several IPI name numbers,
// or with no number at all. Songlinks groups those registrations into one person, shows the evidence for every join,
// and keeps apart the ones it isn't sure of. The API is free, with no key and no sign-up.
//
//	client := songlinks.New(songlinks.WithUserAgent("my-app/1.0"))
//	answer, err := client.Lookup(ctx, "mark hoppus", nil)
//	if err != nil {
//		return err
//	}
//	if answer.Answer == "rejected" { // for example a one-word name, which matches too many people
//		return fmt.Errorf("no answer: %s", answer.Rejection.Code)
//	}
//	fmt.Println(answer.Candidates[0].PersistentID) // SLP-ZYM7-GD3E-S
//
// # Reading an answer
//
// Two words come up throughout. A person is a songwriter Songlinks has given a person ID (SLP-…), with the
// registrations it holds as theirs; [Client.Person] reads one. A candidate ([Cluster]) is what a lookup returns: the
// registrations that one name reaches, grouped fresh for that query. A candidate can count more works than its
// person, because a name can reach registrations the person ID lists only as possible; [Client.Person] is the
// authority on what an ID holds.
//
// The answer types are generated from the API's OpenAPI document and named after its schemas, with the API's
// description of every field as its doc comment. A string the API may leave out or send as null is "" (many records
// have no person ID yet, and that is ordinary). A number, boolean or object it may leave out is a pointer, because
// there nil and zero say different things: a nil [WriterAnswer].Works is not known (it is set only when Status is
// "person"), and a false Prerelease is a released ID.
//
// A candidate's PersistentID is the stored person when there is exactly one; when its records belong to more than
// one, PersistentID is empty and StoredPersons lists them. A Prerelease ID may stop resolving, or point at a different
// person, without notice: check it before you store one.
//
// # Errors and retries
//
// An error answer is returned as an [*Error]: branch on its Code, the API's own error code. A withdrawn person ID is a
// [*PersonWithdrawnError], which carries the API's full answer about the ID. The public API allows about 100 requests
// per 5 minutes from one address, across every endpoint, and 20 from VPNs, proxies and hosting providers (which
// includes most cloud servers). 429 and 503 answers and network failures are retried, up to
// two times by default, waiting as long as the API asks when that is a minute or less, or backing off from half a
// second; a longer requested wait is returned at once, in Error.RetryAfter. A cancelled context is never retried.
//
// Beta: answers are unverified machine output from a dated snapshot of registration data (every answer's
// Provenance.MLCSnapshot gives its date). Some are wrong and some are incomplete. Not for rights or royalty decisions.
package songlinks
