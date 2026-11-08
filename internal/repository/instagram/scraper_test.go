package instagram

import "testing"

func TestParseHandlesProviderFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		payload     string
		wantCaption string
		wantFrames  int
	}{
		{
			name:        "flat caption_text",
			payload:     `{"caption_text":"Топ драм: Дюна","thumbnail_url":"https://cdn/x.jpg"}`,
			wantCaption: "Топ драм: Дюна", wantFrames: 1,
		},
		{
			name:        "nested data object",
			payload:     `{"data":{"caption":"Брат 1997","display_url":"https://cdn/y.jpg","username":"kino"}}`,
			wantCaption: "Брат 1997", wantFrames: 1,
		},
		{
			name: "scrapecreators graphql media",
			payload: `{"success":true,"data":{"xdt_shortcode_media":{
				"display_url":"https://cdn/z.jpg","owner":{"username":"filmcooore"},
				"edge_media_to_caption":{"edges":[{"node":{"text":"«Начало» (2010)"}}]},
				"edge_media_preview_comment":{"edges":[{"node":{"text":"Паприка"}}]}}}}`,
			wantCaption: "«Начало» (2010)", wantFrames: 1,
		},
		{
			name:        "description fallback",
			payload:     `{"description":"Fight Club"}`,
			wantCaption: "Fight Club", wantFrames: 0,
		},
		{
			name:        "empty payload yields nothing",
			payload:     `{}`,
			wantCaption: "", wantFrames: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parse([]byte(tt.payload))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Caption != tt.wantCaption {
				t.Fatalf("caption = %q, want %q", got.Caption, tt.wantCaption)
			}
			if len(got.FrameURLs) != tt.wantFrames {
				t.Fatalf("frames = %d, want %d", len(got.FrameURLs), tt.wantFrames)
			}
		})
	}
}
