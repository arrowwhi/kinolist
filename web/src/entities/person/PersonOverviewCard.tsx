import type { PersonOverview } from '../../api/people'
import { personalAverage } from '../director/metadata'
import { PersonAvatar } from './PersonAvatar'
import { ProgressBar } from './ProgressBar'
import { lifespan, progress, worksWatched } from './metadata'
// Карточка в списках «Мои режиссёры» и «Мои актёры».
export function PersonOverviewCard({
  person,
  onOpen,
}: {
  person: PersonOverview
  onOpen: () => void
}) {
  const percent = progress(person.watched_count, person.work_count)
  return (
    <button type="button" className="person-overview" onClick={onOpen}>
      <span className="person-overview__head">
        <PersonAvatar name={person.name} url={person.photo_url} />
        <span className="person-overview__name">
          <strong>{person.name}</strong>
          <span className="muted">
            {[person.original_name, lifespan(person)]
              .filter(Boolean)
              .join(' · ')}
          </span>
        </span>
        {person.followed && (
          <span className="person-badge" aria-label="Отслеживаю">
            ✓
          </span>
        )}
      </span>
      {person.filmography_complete ? (
        <span className="person-overview__progress">
          <span className="person-overview__progress-label">
            <span>{worksWatched(person.watched_count, person.work_count)}</span>
            <span className="muted">{percent}%</span>
          </span>
          <ProgressBar
            value={percent}
            label={worksWatched(person.watched_count, person.work_count)}
          />
        </span>
      ) : (
        <span className="person-overview__progress-label">
          <span>
            {person.watched_count} просмотрено · {person.in_collection_count} в
            коллекции
          </span>
          <span className="muted">Фильмография загружается</span>
        </span>
      )}
      <span className="person-overview__meta">
        {[
          person.average_user_rating == null
            ? 'Пока без моих оценок'
            : `Моя средняя ${personalAverage(person.average_user_rating)}`,
          person.want_to_watch_count
            ? `${person.want_to_watch_count} на потом`
            : '',
        ]
          .filter(Boolean)
          .join(' · ')}
      </span>
    </button>
  )
}
