import type { UserMovie } from '../api/types'
import { Poster } from './Poster'

interface MovieCardProps {
  item: UserMovie
  onOpen: (item: UserMovie) => void
}

export function MovieCard({ item, onOpen }: MovieCardProps) {
  const { movie } = item
  const meta = [
    movie.year ? String(movie.year) : null,
    movie.rating ? `⭐ ${movie.rating.toFixed(1)}` : null,
    movie.genres.length ? movie.genres.map((g) => g.name_ru).join(', ') : null,
  ].filter(Boolean) as string[]

  return (
    <button className="card" onClick={() => onOpen(item)} type="button">
      <Poster className="card__poster" url={movie.poster_url} alt={movie.title} />

      <div className="card__body">
        <div className="card__title">{movie.title}</div>
        {meta.length > 0 && (
          <div className="card__meta">
            {meta.map((m) => (
              <span key={m}>{m}</span>
            ))}
          </div>
        )}
        {movie.overview && <div className="card__overview">{movie.overview}</div>}
        {item.status === 'watched' && (
          <span className="card__badge">
            Посмотрено{item.user_rating ? ` · ${item.user_rating}/10` : ''}
          </span>
        )}
      </div>
    </button>
  )
}
