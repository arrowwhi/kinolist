import { useState } from 'react'
import { RequestError } from '../../api/client'
import { directorsApi, filmographyKey } from '../../api/directors'
import type { FilmographyItem } from '../../api/directors'
import { catalogPath } from '../../shared/lib/navigation'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { Chip } from '../../shared/ui/Chip'
import { Button } from '../../shared/ui/Button'
import {
  EmptyState,
  ErrorState,
  InlineError,
  SkeletonList,
} from '../../shared/ui/Feedback'
import { useResource } from '../../shared/hooks/useResource'
import { useMutation } from '../../shared/hooks/useMutation'
import { haptic } from '../../shared/lib/telegram'
import { personalAverage } from '../../entities/director/metadata'
import { DirectorAvatar } from '../../entities/director-v2/DirectorAvatar'
import { FilmographyRow } from '../../entities/director-v2/FilmographyRow'
import { ProgressBar } from '../../entities/director-v2/ProgressBar'
import {
  filmCount,
  personLine,
  progress,
  worksWatched,
} from '../../entities/director-v2/metadata'

export type FilmographyFilter = 'all' | 'watched' | 'want_to_watch' | 'missing'
export type FilmographySort = 'year' | 'rating' | 'user_rating'
export interface FilmographyPreferences {
  filter: FilmographyFilter
  sort: FilmographySort
  descending: boolean
}
export const defaultFilmography: FilmographyPreferences = {
  filter: 'all',
  sort: 'year',
  descending: true,
}

const sortLabels: Record<FilmographySort, string> = {
  year: 'По году',
  rating: 'По рейтингу',
  user_rating: 'По моей оценке',
}

// Работы без значения уходят в конец при любом направлении сортировки.
function sortWorks(items: FilmographyItem[], prefs: FilmographyPreferences) {
  const value = (item: FilmographyItem) =>
    prefs.sort === 'year'
      ? item.year
      : prefs.sort === 'rating'
        ? item.rating
        : item.user_rating
  const sign = prefs.descending ? -1 : 1
  return [...items].sort((a, b) => {
    const left = value(a)
    const right = value(b)
    if (left == null && right == null) return (b.year ?? 0) - (a.year ?? 0)
    if (left == null) return 1
    if (right == null) return -1
    return sign * (left - right) || (b.year ?? 0) - (a.year ?? 0)
  })
}

function decade(year?: number) {
  return year ? `${Math.floor(year / 10) * 10}-е` : 'Без года'
}

export function DirectorProfilePage({
  id,
  preferences,
  onPreferences,
  navigate,
  back,
  changed,
}: {
  id: number
  preferences: FilmographyPreferences
  onPreferences: (value: FilmographyPreferences) => void
  navigate: Navigate
  back: () => void
  changed: (message: string) => void
}) {
  const resource = useResource((signal) => directorsApi.profile(id, signal), id)
  // Локальные правки поверх загруженного профиля: подписка и добавленные работы.
  const [followed, setFollowed] = useState<boolean | null>(null)
  const [added, setAdded] = useState<
    Record<string, Pick<FilmographyItem, 'user_movie_id' | 'status'>>
  >({})
  const [adding, setAdding] = useState<string | null>(null)
  const [bioOpen, setBioOpen] = useState(false)
  const follow = useMutation()
  const add = useMutation()

  if (resource.loading || resource.error || !resource.data)
    return (
      <Shell title="Режиссёр" navigate={navigate} onBack={back}>
        {resource.loading ? (
          <SkeletonList />
        ) : (
          <ErrorState
            message={resource.error || 'Режиссёр не найден'}
            onRetry={resource.retry}
          />
        )}
      </Shell>
    )

  const director = resource.data
  const isFollowed = followed ?? director.followed
  const works = director.filmography.map((w) => ({
    ...w,
    ...added[filmographyKey(w)],
  }))
  const watched = works.filter((w) => w.status === 'watched').length
  const want = works.filter((w) => w.status === 'want_to_watch').length
  const missing = works.filter((w) => !w.status).length
  const percent = progress(watched, works.length)
  const visible = sortWorks(
    works.filter((w) =>
      preferences.filter === 'all'
        ? true
        : preferences.filter === 'missing'
          ? !w.status
          : w.status === preferences.filter,
    ),
    preferences,
  )
  const groupByDecade = preferences.sort === 'year'

  const toggleFollow = () =>
    follow.execute(
      () =>
        directorsApi.follow(director.id, !isFollowed).then(() => !isFollowed),
      (value) => {
        setFollowed(value)
        changed(
          value
            ? `${director.name} — в отслеживаемых`
            : `Больше не отслеживаешь: ${director.name}`,
        )
      },
    )
  const addWork = (item: FilmographyItem) => {
    const key = filmographyKey(item)
    setAdding(key)
    void add
      .execute(
        async () => {
          try {
            return await directorsApi.addToWatchlist(item)
          } catch (error) {
            // Работа уже в коллекции (добавлена в другом месте) — просто
            // перечитываем профиль, чтобы показать её настоящий статус.
            if (error instanceof RequestError && error.existingUserMovieId) {
              resource.retry()
              return null
            }
            throw error
          }
        },
        (entry) => {
          if (!entry) return
          setAdded((prev) => ({
            ...prev,
            [key]: { user_movie_id: entry.id, status: entry.status },
          }))
          changed(`«${item.title}» — в «На потом»`)
        },
      )
      .finally(() => setAdding(null))
  }
  const chooseSort = (sort: FilmographySort) => {
    haptic('select')
    onPreferences(
      sort === preferences.sort
        ? { ...preferences, descending: !preferences.descending }
        : { ...preferences, sort, descending: true },
    )
  }

  let lastDecade = ''
  return (
    <Shell
      title={director.name}
      subtitle={personLine(director)}
      navigate={navigate}
      onBack={back}
    >
      <section className="director-hero" aria-label="О режиссёре">
        <DirectorAvatar
          name={director.name}
          url={director.photo_url}
          size="lg"
        />
        <div className="director-hero__body">
          {director.place_of_birth && (
            <p className="small">
              <span className="muted">
                {director.death_date ? 'Родился' : 'Место рождения'}:{' '}
              </span>
              {director.place_of_birth}
            </p>
          )}
          {director.known_for.length > 0 && (
            <p className="small">
              <span className="muted">Известен по: </span>
              {director.known_for.join(', ')}
            </p>
          )}
          <Button
            variant={isFollowed ? 'secondary' : 'primary'}
            className="director-hero__follow"
            busy={follow.busy}
            aria-pressed={isFollowed}
            onClick={toggleFollow}
          >
            {isFollowed ? '✓ Отслеживаю' : '＋ Отслеживать'}
          </Button>
        </div>
      </section>
      <InlineError message={follow.error} />
      {director.stale && (
        <p className="muted small" role="status">
          Каталог сейчас не отвечает — показываем сохранённую фильмографию.
        </p>
      )}
      {!director.filmography_complete && (
        <p className="muted small" role="status">
          Полная фильмография ещё загружается — пока видны только работы из
          коллекции.
        </p>
      )}

      <section className="director-summary" aria-label="Мой прогресс">
        <div className="director-summary__line">
          <strong>{worksWatched(watched, works.length)}</strong>
          <span className="muted">{percent}%</span>
        </div>
        <ProgressBar
          value={percent}
          label={worksWatched(watched, works.length)}
        />
        <dl className="director-summary__stats">
          <div>
            <dt>Моя средняя</dt>
            <dd>{personalAverage(director.average_user_rating)}</dd>
          </div>
          <div>
            <dt>На потом</dt>
            <dd>{want}</dd>
          </div>
          <div>
            <dt>Всего</dt>
            <dd>{works.length}</dd>
          </div>
        </dl>
      </section>

      {director.biography && (
        <section className="stack stack--tight" aria-label="Биография">
          <h2>Биография</h2>
          <p
            className={`director-bio ${bioOpen ? '' : 'director-bio--clamped'}`}
          >
            {director.biography}
          </p>
          <button
            type="button"
            className="text-toggle"
            aria-expanded={bioOpen}
            onClick={() => setBioOpen(!bioOpen)}
          >
            {bioOpen ? 'Свернуть' : 'Читать полностью'}
          </button>
        </section>
      )}

      <section className="stack" aria-labelledby="filmography-title">
        <div className="filmography-heading">
          <h2 id="filmography-title">Фильмография</h2>
          <span className="muted small">{filmCount(works.length)}</span>
        </div>
        <div className="chips" role="group" aria-label="Какие работы показать">
          {(
            [
              { key: 'all', label: 'Все', count: works.length },
              { key: 'watched', label: 'Просмотрено', count: watched },
              { key: 'want_to_watch', label: 'На потом', count: want },
              { key: 'missing', label: 'Не в коллекции', count: missing },
            ] as const
          ).map((o) => (
            <Chip
              key={o.key}
              active={preferences.filter === o.key}
              onClick={() => onPreferences({ ...preferences, filter: o.key })}
            >
              {o.label}
              <span className="chip__count">{o.count}</span>
            </Chip>
          ))}
        </div>
        <div className="sort-row" role="group" aria-label="Сортировка работ">
          <span className="muted small">Сортировка</span>
          <div className="chips">
            {(Object.keys(sortLabels) as FilmographySort[]).map((key) => {
              const active = preferences.sort === key
              return (
                <Chip
                  key={key}
                  active={active}
                  onClick={() => chooseSort(key)}
                  aria-label={
                    active
                      ? `${sortLabels[key]}, ${preferences.descending ? 'по убыванию' : 'по возрастанию'}. Нажми, чтобы развернуть`
                      : sortLabels[key]
                  }
                >
                  {sortLabels[key]}
                  {active && (
                    <span className="chip__arrow" aria-hidden="true">
                      {preferences.descending ? '↓' : '↑'}
                    </span>
                  )}
                </Chip>
              )
            })}
          </div>
        </div>
        {!visible.length ? (
          <EmptyState
            title="Ничего не нашлось"
            description={
              preferences.filter === 'missing'
                ? 'Все работы режиссёра уже в твоей коллекции.'
                : 'В этой подборке пока нет работ.'
            }
            action="Показать все"
            onAction={() => onPreferences({ ...preferences, filter: 'all' })}
          />
        ) : (
          <ul className="filmography" aria-live="polite">
            {visible.map((item) => {
              const label = decade(item.year)
              const header = groupByDecade && label !== lastDecade
              lastDecade = label
              return [
                header && (
                  <li key={`d:${label}`} className="filmography__decade">
                    {label}
                  </li>
                ),
                <FilmographyRow
                  key={filmographyKey(item)}
                  item={item}
                  adding={adding === filmographyKey(item)}
                  onOpen={(work) =>
                    navigate(
                      work.user_movie_id != null
                        ? `/movie/${work.user_movie_id}`
                        : catalogPath({
                            provider: 'tmdb',
                            mediaType: work.media_type,
                            externalId: work.tmdb_id!,
                            directorId: director.id,
                          }),
                    )
                  }
                  onAdd={addWork}
                />,
              ]
            })}
          </ul>
        )}
        <InlineError message={add.error} />
      </section>
    </Shell>
  )
}
