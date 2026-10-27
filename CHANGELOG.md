# Changelog

## v0.2.0

- `Songwriters`: who wrote a recording, by ISRC. Up to 50 ISRCs per call (`SongwritersMax`); for each, the works it
  is registered to, their writers, and each writer's person ID where there is one. Answers are `RecordingAnswer`,
  `RecordingWork` and `RecordingWriter`, generated from the API's document like the rest.
- `NormalizeISRC`, to match `Songwriters` answers to the codes you sent, as `NormalizeIPI` does for `Writers`.
- New example, `examples/songwriters-by-isrc`: a list of ISRCs in, CSV out.
- `PersonURL` fields now say they are relative to https://songlinks.io, rather than "this site".

## v0.1.1

- Field documentation regenerated from the API's current description of itself: "IPI name number" throughout, what
  a person's member records are, why a lookup candidate can count more works than its person, and the edge's rate
  limit.

## v0.1.0

First release: `Lookup`, `Person`, `PersonWorks`, `Writers` and `Version` over the Songlinks API 0.11.0, typed
answers, typed errors, and retries that honour `Retry-After`.
