import type { Movie } from '../../api/types'
import { formatRuntime } from '../../shared/lib/format'
import { Poster } from './Poster'
import { Rating } from './Rating'
export function MoviePreview({ movie }: { movie: Movie }) {
  return (
    <>
      <Poster
        url={movie.poster_url}
        title={movie.original_title || movie.title}
        size="wide"
      />
      <h2 className="preview-title">{movie.title}</h2>
      <p className="muted small">
        {[
          movie.original_title !== movie.title ? movie.original_title : '',
          movie.year,
          formatRuntime(movie.runtime_min),
        ]
          .filter(Boolean)
          .join(' · ')}
      </p>
      <div className="preview-metadata">
        <Rating movie={movie} />
        {movie.genres.map((g) => (
          <span className="metadata-tag" key={g.id}>
            {g.name_ru}
          </span>
        ))}
      </div>
      <p className="muted overview">
        {movie.overview || 'В каталоге пока нет описания.'}
      </p>
    </>
  )
}
