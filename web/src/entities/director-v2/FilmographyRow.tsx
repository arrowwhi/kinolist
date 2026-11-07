import type { FilmographyItem } from '../../api/directors'
import { Poster } from '../movie/Poster'
export function FilmographyRow({
  item,
  adding,
  onOpen,
  onAdd,
}: {
  item: FilmographyItem
  adding: boolean
  onOpen: (userMovieId: number) => void
  onAdd: (item: FilmographyItem) => void
}) {
  const owned = item.user_movie_id != null
  // Работу без TMDB id в коллекцию одной кнопкой не добавить.
  const addable = item.tmdb_id != null
  const body = (
    <>
      <Poster url={item.poster_url} title={item.title} />
      <span className="film-row__body">
        <span className="film-row__title">{item.title}</span>
        <span className="film-row__meta">
          {[
            item.year ?? 'Год неизвестен',
            item.media_type === 'tv'
              ? item.episode_count
                ? `Сериал · ${item.episode_count} эп.`
                : 'Сериал'
              : 'Фильм',
            item.original_title !== item.title ? item.original_title : '',
          ]
            .filter(Boolean)
            .join(' · ')}
        </span>
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
      {owned ? (
        <button
          type="button"
          className="film-row__main"
          onClick={() => onOpen(item.user_movie_id!)}
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
