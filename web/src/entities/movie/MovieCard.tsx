import type { UserMovie } from '../../api/types'
import { formatDate } from '../../shared/lib/format'
import { Poster } from './Poster'
import { Rating } from './Rating'
import { movieMetadata } from './metadata'
export function MovieCard({
  item,
  onOpen,
}: {
  item: UserMovie
  onOpen: (id: number) => void
}) {
  const watched = item.status === 'watched'
  return (
    <button
      className="movie-card"
      onClick={() => onOpen(item.id)}
      type="button"
    >
      <Poster url={item.movie.poster_url} title={item.movie.title} />
      <span className="movie-card__body">
        <span className="movie-card__title">{item.movie.title}</span>
        <span className="movie-card__meta">
          {[
            movieMetadata(item.movie),
            item.movie.media_type === 'tv'
              ? ''
              : item.movie.genres
                  .slice(0, 2)
                  .map((g) => g.name_ru)
                  .join(', '),
          ]
            .filter(Boolean)
            .join(' · ')}
        </span>
        {watched ? (
          <span className="movie-card__personal">
            {item.user_rating
              ? `Моя оценка ${item.user_rating}/10`
              : 'Без оценки'}
            {item.review ? ' · Есть отзыв' : ''}
          </span>
        ) : (
          <span className="movie-card__rating">
            <Rating movie={item.movie} />
            {item.note && <span> · Есть заметка</span>}
          </span>
        )}
        <span className="movie-card__date">
          {watched
            ? `Просмотрено ${formatDate(item.watched_at)}`
            : `Добавлено ${formatDate(item.added_at)}`}
        </span>
        {watched && item.review && (
          <span className="movie-card__review">{item.review}</span>
        )}
      </span>
    </button>
  )
}
