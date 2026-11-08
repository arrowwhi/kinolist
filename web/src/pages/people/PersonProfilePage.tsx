import { useState } from 'react'
import { RequestError } from '../../api/client'
import { filmographyKey } from '../../api/people'
import type { FilmographyItem } from '../../api/people'
import { catalogPath } from '../../shared/lib/navigation'
import type { Navigate } from '../../shared/lib/navigation'
import type { PeopleSection } from '../../features/people/sections'
import {
  decade,
  filterWorks,
  sortWorks,
} from '../../features/people/filmography'
import type {
  FilmographyPreferences,
  FilmographySort,
} from '../../features/people/filmography'
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
import { PersonAvatar } from '../../entities/person/PersonAvatar'
import { FilmographyRow } from '../../entities/person/FilmographyRow'
import { ProgressBar } from '../../entities/person/ProgressBar'
import {
  personLine,
  progress,
  worksWatched,
} from '../../entities/person/metadata'

const sortLabels: Record<FilmographySort, string> = {
  year: 'По году',
  rating: 'По рейтингу',
  user_rating: 'По моей оценке',
}

// Экраны «23 · Профиль режиссёра» и «31 · Профиль актёра».
export function PersonProfilePage({
  section,
  id,
  preferences,
  onPreferences,
  navigate,
  back,
  changed,
}: {
  section: PeopleSection
  id: number
  preferences: FilmographyPreferences
  onPreferences: (value: FilmographyPreferences) => void
  navigate: Navigate
  back: () => void
  changed: (message: string) => void
}) {
  const resource = useResource(
    (signal) => section.api.profile(id, signal),
    `${section.kind}:${id}`,
  )
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
      <Shell
        title={section.profile.fallbackTitle}
        navigate={navigate}
        onBack={back}
      >
        {resource.loading ? (
          <SkeletonList />
        ) : (
          <ErrorState
            message={resource.error || section.profile.notFound}
            onRetry={resource.retry}
          />
        )}
      </Shell>
    )

  const person = resource.data
  const isFollowed = followed ?? person.followed
  const works = person.filmography.map((w) => ({
    ...w,
    ...added[filmographyKey(w)],
  }))
  const watched = works.filter((w) => w.status === 'watched').length
  const want = works.filter((w) => w.status === 'want_to_watch').length
  const missing = works.filter((w) => !w.status).length
  const percent = progress(watched, works.length)
  const visible = sortWorks(filterWorks(works, preferences.filter), preferences)
  const groupByDecade = preferences.sort === 'year'

  const toggleFollow = () =>
    follow.execute(
      () => section.api.follow(person.id, !isFollowed).then(() => !isFollowed),
      (value) => {
        setFollowed(value)
        changed(
          value
            ? `${person.name} — в отслеживаемых`
            : `Больше не отслеживаешь: ${person.name}`,
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
            return await section.api.addToWatchlist(item)
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
      title={person.name}
      subtitle={personLine(person)}
      navigate={navigate}
      onBack={back}
    >
      <section className="person-hero" aria-label={section.profile.heroLabel}>
        <PersonAvatar name={person.name} url={person.photo_url} size="lg" />
        <div className="person-hero__body">
          {person.place_of_birth && (
            <p className="small">
              <span className="muted">
                {person.death_date ? 'Родился' : 'Место рождения'}:{' '}
              </span>
              {person.place_of_birth}
            </p>
          )}
          {person.known_for.length > 0 && (
            <p className="small">
              <span className="muted">Известен по: </span>
              {person.known_for.join(', ')}
            </p>
          )}
          <Button
            variant={isFollowed ? 'secondary' : 'primary'}
            className="person-hero__follow"
            busy={follow.busy}
            aria-pressed={isFollowed}
            onClick={toggleFollow}
          >
            {isFollowed ? '✓ Отслеживаю' : '＋ Отслеживать'}
          </Button>
        </div>
      </section>
      <InlineError message={follow.error} />
      {person.stale && (
        <p className="muted small" role="status">
          {section.profile.stale}
        </p>
      )}
      {!person.filmography_complete && (
        <p className="muted small" role="status">
          {section.profile.loading}
        </p>
      )}

      <section className="person-summary" aria-label="Мой прогресс">
        <div className="person-summary__line">
          <strong>{worksWatched(watched, works.length)}</strong>
          <span className="muted">{percent}%</span>
        </div>
        <ProgressBar
          value={percent}
          label={worksWatched(watched, works.length)}
        />
        <dl className="person-summary__stats">
          <div>
            <dt>Моя средняя</dt>
            <dd>{personalAverage(person.average_user_rating)}</dd>
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

      {person.biography && (
        <section className="stack stack--tight" aria-label="Биография">
          <h2>Биография</h2>
          <p className={`person-bio ${bioOpen ? '' : 'person-bio--clamped'}`}>
            {person.biography}
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
          <h2 id="filmography-title">{section.profile.worksTitle}</h2>
          <span className="muted small">
            {section.profile.worksCount(works.length)}
          </span>
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
                ? section.profile.allInCollection
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
                  meta={section.workMeta(item)}
                  adding={adding === filmographyKey(item)}
                  onOpen={(work) =>
                    navigate(
                      work.user_movie_id != null
                        ? `/movie/${work.user_movie_id}`
                        : catalogPath({
                            provider: 'tmdb',
                            mediaType: work.media_type,
                            externalId: work.tmdb_id!,
                            ...section.catalogSource(person.id),
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
