package movie

// Director identity is scoped to a catalogue, never inferred from a name.
type Director struct {
	ID           int64
	TMDBID       int32
	KinopoiskID  int32
	Name         string
	OriginalName string
}

type DirectorStats struct {
	Director
	TotalCount        int
	WantToWatchCount  int
	WatchedCount      int
	RatedCount        int
	AverageUserRating *float64
}

type DirectorFilter struct {
	UserID    int64
	MediaType MediaType
	Query     string
	Sort      string
	Desc      bool
	Limit     int
	Offset    int
}

func (f DirectorFilter) Normalize() DirectorFilter {
	page := (Filter{Limit: f.Limit, Offset: f.Offset}).Normalize()
	f.Limit, f.Offset = page.Limit, page.Offset
	if f.Sort == "" {
		f.Sort = "name"
	}
	return f
}
