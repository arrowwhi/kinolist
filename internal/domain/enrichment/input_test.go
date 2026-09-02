package enrichment_test

import (
	"errors"
	"testing"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
)

func TestClassifyInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		wantKind  enrichment.Kind
		wantValue string
		wantErr   error
	}{
		{
			name: "plain title", in: "Бойцовский клуб",
			wantKind: enrichment.KindTitle, wantValue: "Бойцовский клуб",
		},
		{
			name: "title is trimmed", in: "  Дюна  ",
			wantKind: enrichment.KindTitle, wantValue: "Дюна",
		},
		{
			name: "reel url", in: "https://www.instagram.com/reel/Cx1y2Z3aBcD/",
			wantKind: enrichment.KindReel, wantValue: "https://www.instagram.com/reel/Cx1y2Z3aBcD/",
		},
		{
			name: "tracking params are stripped", in: "https://www.instagram.com/reel/Abc123/?igsh=xyz&utm_source=ig_web",
			wantKind: enrichment.KindReel, wantValue: "https://www.instagram.com/reel/Abc123/",
		},
		{
			name: "post url is accepted", in: "https://instagram.com/p/Abc123",
			wantKind: enrichment.KindReel, wantValue: "https://instagram.com/p/Abc123",
		},
		{
			name: "schemeless host is accepted", in: "instagram.com/reel/Abc123",
			wantKind: enrichment.KindReel, wantValue: "https://instagram.com/reel/Abc123",
		},
		{
			name: "movie title containing a dot is not a url", in: "8 миля",
			wantKind: enrichment.KindTitle, wantValue: "8 миля",
		},
		{name: "empty input", in: "   ", wantErr: enrichment.ErrEmptyInput},
		{name: "foreign host", in: "https://tiktok.com/@user/video/123", wantErr: enrichment.ErrInvalidReelURL},
		{name: "instagram profile is not a reel", in: "https://instagram.com/someuser", wantErr: enrichment.ErrInvalidReelURL},
		{name: "instagram root", in: "https://instagram.com/", wantErr: enrichment.ErrInvalidReelURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kind, value, err := enrichment.ClassifyInput(tt.in)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if kind != tt.wantKind {
				t.Fatalf("kind = %q, want %q", kind, tt.wantKind)
			}
			if value != tt.wantValue {
				t.Fatalf("value = %q, want %q", value, tt.wantValue)
			}
		})
	}
}
