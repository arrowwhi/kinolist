package movie

import "testing"

func TestSameTitleAndYearAcrossTypesRequiresChoice(t *testing.T) {
	got := SelectMatch("Shared", 2022, []Candidate{
		{Provider: ProviderTMDB, ExternalID: 7, Title: "Shared", Year: 2022, MediaType: MediaMovie},
		{Provider: ProviderTMDB, ExternalID: 7, Title: "Shared", Year: 2022, MediaType: MediaTV},
	})
	if got.Kind != MatchAmbiguous || len(got.Candidates) != 2 {
		t.Fatalf("match: %+v", got)
	}
}
