import { contentCount, mediaFilterLabel } from '../../entities/movie/metadata'
import type { MediaType, MovieList } from '../../api/types'
export function CollectionStats({
  summary,
  filterLabel,
  mediaType,
}: {
  summary: MovieList['summary']
  mediaType?: MediaType
  filterLabel: string
}) {
  return (
    <div className="stats-block">
      <p className="muted small">
        По текущим фильтрам · {mediaFilterLabel(mediaType)} · {filterLabel}
      </p>
      <div
        className="stats"
        aria-label="Статистика по фильтрам без учёта поиска"
      >
        <span>{contentCount(summary.movie_count, mediaType)}</span>
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
