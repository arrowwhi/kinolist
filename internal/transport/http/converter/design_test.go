package converter

import (
	"encoding/json"
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	"github.com/arrowwhi/kinolist/internal/usecase/collection"
)

func TestUnknownStatisticsEncodeAsNull(t *testing.T) {
	b, err := json.Marshal(MovieList(collection.Page{}))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	summary := got["summary"].(map[string]any)
	if summary["average_user_rating"] != nil {
		t.Fatal(string(b))
	}
	if _, ok := summary["average_user_rating"]; !ok {
		t.Fatal("missing explicit null")
	}
}

func TestReviewAndRatingMetadata(t *testing.T) {
	review := "A review"
	note := "A note"
	upd := Update(dto.UpdateMovieRequest{Review: &review, Note: &note})
	if *upd.Review != review || *upd.Note != note {
		t.Fatal(upd)
	}
	votes := int32(42)
	m := Movie(movie.Movie{Rating: 8, RatingProvider: movie.ProviderTMDB, RatingVoteCount: &votes, RuntimeMin: 166})
	if m.RatingProvider != "tmdb" || *m.RatingVoteCount != 42 || m.RuntimeMin != 166 {
		t.Fatal(m)
	}
}
