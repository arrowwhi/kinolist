package converter

import (
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/transport/http/dto"
	actorsuc "github.com/arrowwhi/kinolist/internal/usecase/actors"
)

// ActorOverview — карточка актёра; поля те же, что у режиссёра.
func ActorOverview(a movie.ActorOverview) dto.ActorOverview {
	return DirectorOverview(a)
}

func ActorOverviewList(p actorsuc.Page) dto.ActorOverviewList {
	items := make([]dto.ActorOverview, 0, len(p.Items))
	for _, a := range p.Items {
		items = append(items, ActorOverview(a))
	}
	return dto.ActorOverviewList{
		Items: items, Total: p.Total, Limit: p.Limit, Offset: p.Offset,
		Counts: dto.DirectorScopeCounts{All: p.Counts.All, Followed: p.Counts.Followed, Collection: p.Counts.Collection},
	}
}

func ActorProfile(p movie.ActorProfile, now time.Time) dto.ActorProfile {
	items := make([]dto.ActorFilmographyItem, 0, len(p.Filmography))
	for _, w := range p.Filmography {
		items = append(items, dto.ActorFilmographyItem{FilmographyItem: filmographyItem(w, now), Character: w.Character})
	}
	return dto.ActorProfile{
		ActorOverview: ActorOverview(p.DirectorOverview),
		Biography:     p.Biography, BiographyLang: p.BiographyLang, Stale: p.Stale, Filmography: items,
	}
}
