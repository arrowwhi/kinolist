import type { Movie } from '../../api/types'
export function Rating({
  movie,
  large = false,
}: {
  movie: Movie
  large?: boolean
}) {
  if (!movie.rating) return <span className="muted">Нет оценки каталога</span>
  const provider =
    movie.rating_provider === 'tmdb'
      ? 'TMDB'
      : movie.rating_provider === 'kinopoisk'
        ? 'Кинопоиск'
        : 'Рейтинг каталога'
  const votes = movie.rating_vote_count
  return (
    <div
      className={
        large ? 'catalog-rating catalog-rating--large' : 'catalog-rating'
      }
    >
      <span className="catalog-rating__value">{movie.rating.toFixed(1)}</span>
      <span className="catalog-rating__source">
        {provider}
        {votes != null
          ? ` · ${new Intl.NumberFormat('ru-RU', { notation: 'compact', maximumFractionDigits: 1 }).format(votes)} оценок`
          : ''}
      </span>
    </div>
  )
}
