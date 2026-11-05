import { ratedWorks, workCount } from '../../entities/director/metadata'
import { contentCount, mediaFilterLabel } from '../../entities/movie/metadata'
import type { MediaType, MovieList } from '../../api/types'
export function CollectionStats({
  summary,
  filterLabel,
  mediaType,
  showRatedCount = false,
}: {
  summary: MovieList['summary']
  mediaType?: MediaType
  showRatedCount?: boolean
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
        <span>
          {showRatedCount
            ? workCount(summary.movie_count)
            : contentCount(summary.movie_count, mediaType)}
        </span>
        <span className="accent">
          {summary.average_user_rating == null
            ? '—'
            : summary.average_user_rating.toLocaleString('ru-RU', {
                minimumFractionDigits: 1,
                maximumFractionDigits: 1,
              })}
          <small>
            {summary.average_user_rating == null
              ? 'пока нет оценок'
              : showRatedCount
                ? 'моя средняя оценка'
                : 'средняя оценка'}
          </small>
        </span>
      </div>
      {showRatedCount && (
        <p className="muted small">{ratedWorks(summary.rated_count)}</p>
      )}
    </div>
  )
}
