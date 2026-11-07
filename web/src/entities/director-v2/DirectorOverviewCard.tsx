import type { DirectorOverview } from '../../api/directors'
import { personalAverage } from '../director/metadata'
import { DirectorAvatar } from './DirectorAvatar'
import { ProgressBar } from './ProgressBar'
import { lifespan, progress, worksWatched } from './metadata'
export function DirectorOverviewCard({
  director,
  onOpen,
}: {
  director: DirectorOverview
  onOpen: () => void
}) {
  const percent = progress(director.watched_count, director.work_count)
  return (
    <button type="button" className="director-overview" onClick={onOpen}>
      <span className="director-overview__head">
        <DirectorAvatar name={director.name} url={director.photo_url} />
        <span className="director-overview__name">
          <strong>{director.name}</strong>
          <span className="muted">
            {[director.original_name, lifespan(director)]
              .filter(Boolean)
              .join(' · ')}
          </span>
        </span>
        {director.followed && (
          <span className="director-badge" aria-label="Отслеживаю">
            ✓
          </span>
        )}
      </span>
      {director.filmography_complete ? (
        <span className="director-overview__progress">
          <span className="director-overview__progress-label">
            <span>
              {worksWatched(director.watched_count, director.work_count)}
            </span>
            <span className="muted">{percent}%</span>
          </span>
          <ProgressBar
            value={percent}
            label={worksWatched(director.watched_count, director.work_count)}
          />
        </span>
      ) : (
        <span className="director-overview__progress-label">
          <span>
            {director.watched_count} просмотрено ·{' '}
            {director.in_collection_count} в коллекции
          </span>
          <span className="muted">Фильмография загружается</span>
        </span>
      )}
      <span className="director-overview__meta">
        {[
          director.average_user_rating == null
            ? 'Пока без моих оценок'
            : `Моя средняя ${personalAverage(director.average_user_rating)}`,
          director.want_to_watch_count
            ? `${director.want_to_watch_count} на потом`
            : '',
        ]
          .filter(Boolean)
          .join(' · ')}
      </span>
    </button>
  )
}
