import { movieCount } from '../../shared/lib/format'
import type { MovieList } from '../../api/types'
export function CollectionStats({
  summary,
  filterLabel,
}: {
  summary: MovieList['summary']
  filterLabel: string
}) {
  return (
    <div className="stats-block">
      <p className="muted small">По текущим фильтрам · {filterLabel}</p>
      <div
        className="stats"
        aria-label="Статистика по фильтрам без учёта поиска"
      >
        <span>{movieCount(summary.movie_count)}</span>
        <span className="accent">
          {summary.average_user_rating == null
            ? '—'
            : summary.average_user_rating.toFixed(1)}
          <small>
            {summary.average_user_rating == null
              ? 'пока нет оценок'
              : 'средняя оценка'}
          </small>
        </span>
      </div>
    </div>
  )
}
