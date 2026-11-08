import type { FilmographyItem } from '../../api/people'
import { Poster } from '../movie/Poster'
import { workMeta } from './metadata'
export function FilmographyRow<T extends FilmographyItem>({
  item,
  meta = workMeta(item),
  adding,
  onOpen,
  onAdd,
}: {
  item: T
  // Строка под названием; по умолчанию — как у режиссёра.
  meta?: string
  adding: boolean
  // Своя работа открывает карточку коллекции, чужая — карточку каталога.
  onOpen: (item: T) => void
  onAdd: (item: T) => void
}) {
  const owned = item.user_movie_id != null
  // Работу без TMDB id в коллекцию одной кнопкой не добавить.
  const addable = item.tmdb_id != null
  // Работу только из Кинопоиска (без TMDB id) открыть пока негде.
  const openable = owned || addable
  const body = (
    <>
      <Poster url={item.poster_url} title={item.title} />
      <span className="film-row__body">
        <span className="film-row__title">{item.title}</span>
        <span className="film-row__meta">{meta}</span>
        <span className="film-row__rating">
          {item.upcoming ? (
            <span className="film-row__tag">Скоро</span>
          ) : item.rating != null ? (
            <>
              <span className="film-row__score">{item.rating.toFixed(1)}</span>
              <span className="muted">TMDB</span>
            </>
          ) : (
            <span className="muted">Нет оценки</span>
          )}
        </span>
      </span>
    </>
  )
  return (
    <li className={`film-row ${owned ? 'film-row--owned' : ''}`}>
      {openable ? (
        <button
          type="button"
          className="film-row__main"
          onClick={() => onOpen(item)}
        >
          {body}
        </button>
      ) : (
        <div className="film-row__main">{body}</div>
      )}
      <span className="film-row__side">
        {item.status === 'watched' ? (
          <span
            className="film-status film-status--watched"
            aria-label={
              item.user_rating
                ? `Просмотрено, моя оценка ${item.user_rating}`
                : 'Просмотрено'
            }
          >
            {item.user_rating ? `★ ${item.user_rating}` : '✓'}
          </span>
        ) : item.status === 'want_to_watch' ? (
          <span className="film-status">На потом</span>
        ) : addable ? (
          <button
            type="button"
            className="film-add"
            aria-label={`Добавить «${item.title}» в «На потом»`}
            aria-busy={adding || undefined}
            disabled={adding}
            onClick={() => onAdd(item)}
          >
            {adding ? (
              <span className="spinner" aria-hidden="true" />
            ) : (
              <svg
                viewBox="0 0 24 24"
                width="18"
                height="18"
                aria-hidden="true"
              >
                <path
                  d="M12 5v14M5 12h14"
                  stroke="currentColor"
                  strokeWidth="2.4"
                  strokeLinecap="round"
                />
              </svg>
            )}
          </button>
        ) : null}
      </span>
    </li>
  )
}
