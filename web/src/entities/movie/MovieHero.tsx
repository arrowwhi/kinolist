import type { Genre, Movie } from '../../api/types'
import { mediaLabel, seasonCount } from './metadata'
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
          {[mediaLabel(movie.media_type), movie.year]
            .filter(Boolean)
            .join(' · ')}
          {movie.media_type === 'tv' ? (
            <>
              <br />
              {seasonCount(movie.season_count)}
            </>
          ) : movie.runtime_min ? (
            <>
              <br />
              {formatRuntime(movie.runtime_min)}
            </>
          ) : null}
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
