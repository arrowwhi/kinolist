import type { DirectorStats } from '../../api/types'
import { personalAverage, ratedWorks } from './metadata'
export function DirectorCard({
  director,
  onOpen,
}: {
  director: DirectorStats
  onOpen: () => void
}) {
  return (
    <button type="button" className="director-card" onClick={onOpen}>
      <strong>{director.name}</strong>
      <span className="muted">
        {director.want_to_watch_count} на потом · {director.watched_count}{' '}
        просмотрено
      </span>
      <span className="accent">
        {director.average_user_rating == null
          ? 'Пока нет оценок'
          : `Моя средняя: ${personalAverage(director.average_user_rating)} · ${ratedWorks(director.rated_count).toLocaleLowerCase('ru-RU')}`}
      </span>
      <span className="accent">Открыть работы →</span>
    </button>
  )
}
