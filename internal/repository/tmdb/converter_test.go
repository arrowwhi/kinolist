package tmdb

import (
	"encoding/json"
	"testing"
)

func TestDetailsPreservesRuntimeAndVoteCount(t *testing.T) {
	var d detailsResponse
	if err := json.Unmarshal([]byte(`{"id":1,"title":"Dune","runtime":166,"vote_average":8.5,"vote_count":145000,"genres":[{"id":18,"name":"Drama"}]}`), &d); err != nil {
		t.Fatal(err)
	}
	c := d.toCandidate()
	if c.RuntimeMin != 166 || c.RatingVoteCount == nil || *c.RatingVoteCount != 145000 || c.RatingProvider != "tmdb" || len(c.GenreTMDBIDs) != 1 {
		t.Fatalf("candidate: %+v", c)
	}
}

func TestMissingVoteCountIsUnknown(t *testing.T) {
	if c := (result{ID: 1}).toCandidate(); c.RatingVoteCount != nil {
		t.Fatal("unknown votes must remain nil")
	}
}
