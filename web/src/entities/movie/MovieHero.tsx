import type { Genre, Movie } from '../../api/types'
import { formatRuntime } from '../../shared/lib/format'
import { Poster } from './Poster'
import { Rating } from './Rating'
export function MovieHero({
  movie,
  onGenre,
}: {
  movie: Movie
  onGenre?: (genre: Genre) => void
}) {
  return (
    <div className="movie-hero">
      <Poster url={movie.poster_url} title={movie.title} size="hero" />
      <div className="movie-hero__body">
        <p className="muted small">
          {[movie.year, formatRuntime(movie.runtime_min)]
            .filter(Boolean)
            .join(' · ')}
        </p>
        <Rating movie={movie} large />
        <div className="movie-hero__genres">
          {movie.genres.map((g) =>
            onGenre ? (
              <button type="button" key={g.id} onClick={() => onGenre(g)}>
                {g.name_ru}
              </button>
            ) : (
              <span key={g.id}>{g.name_ru}</span>
            ),
          )}
        </div>
      </div>
    </div>
  )
}
