import { useState } from 'react'
import type { PersonSearchResult } from '../../api/people'
import { personPath } from '../../shared/lib/navigation'
import type { Navigate } from '../../shared/lib/navigation'
import type { PeopleSection } from '../../features/people/sections'
import { Shell } from '../../widgets/app-shell/Shell'
import { Field } from '../../shared/ui/Field'
import { EmptyState, ErrorState, InlineError } from '../../shared/ui/Feedback'
import { useResource } from '../../shared/hooks/useResource'
import { useDebounced } from '../../shared/hooks/useDebounced'
import { useMutation } from '../../shared/hooks/useMutation'
import { PersonAvatar } from '../../entities/person/PersonAvatar'

const departments: Record<string, string> = {
  Directing: 'Режиссёр',
  Acting: 'Актёр',
  Writing: 'Сценарист',
  Production: 'Продюсер',
}

function personMeta(person: PersonSearchResult) {
  return [
    person.original_name !== person.name ? person.original_name : '',
    person.known_for_department
      ? (departments[person.known_for_department] ??
        person.known_for_department)
      : '',
  ]
    .filter(Boolean)
    .join(' · ')
}

// Экраны «25 · Добавить режиссёра» и «32 · Добавить актёра».
export function AddPersonPage({
  section,
  initialQuery,
  navigate,
  back,
  changed,
}: {
  section: PeopleSection
  initialQuery: string
  navigate: Navigate
  back: () => void
  changed: (message: string) => void
}) {
  const [search, setSearch] = useState(initialQuery)
  const text = section.search
  const query = useDebounced(search).trim()
  const resource = useResource(
    (signal) => section.api.search(query, signal),
    `${section.kind}:${query}`,
  )
  // Локальные правки поверх выдачи по tmdb_id: заведённый id и подписка.
  const [local, setLocal] = useState<
    Record<number, { id: number; followed: boolean }>
  >({})
  const [pending, setPending] = useState<number | null>(null)
  const action = useMutation()

  const state = (p: PersonSearchResult) =>
    local[p.tmdb_id] ?? { id: p.id, followed: p.followed }

  const toggle = (person: PersonSearchResult) => {
    const current = state(person)
    const value = !current.followed
    setPending(person.tmdb_id)
    void action
      .execute(
        async () => {
          // Человека, которого ещё нет в справочнике, заводим и подписываем разом.
          if (current.id == null) {
            const d = await section.api.followByTmdb(person.tmdb_id)
            return d.id
          }
          await section.api.follow(current.id, value)
          return current.id
        },
        (id) => {
          setLocal((prev) => ({
            ...prev,
            [person.tmdb_id]: { id, followed: value },
          }))
          changed(
            value
              ? `${person.name} — в отслеживаемых`
              : `Больше не отслеживаешь: ${person.name}`,
          )
        },
      )
      .finally(() => setPending(null))
  }

  const open = (person: PersonSearchResult) => {
    const current = state(person)
    if (current.id != null) {
      navigate(personPath(section.kind, current.id))
      return
    }
    // Профиль строится по нашему id, поэтому сначала импортируем человека.
    setPending(person.tmdb_id)
    void action
      .execute(
        () => section.api.importPerson(person.tmdb_id),
        (d) => navigate(personPath(section.kind, d.id)),
      )
      .finally(() => setPending(null))
  }

  return (
    <Shell
      title={text.title}
      subtitle={text.subtitle}
      navigate={navigate}
      onBack={back}
    >
      <Field
        type="search"
        autoFocus
        placeholder={text.placeholder}
        aria-label={text.label}
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <p className="section-label">
        {query ? 'Результаты поиска' : 'Из твоей коллекции'}
      </p>
      {resource.loading ? (
        <div className="person-list" role="status" aria-label={text.loading}>
          {[1, 2, 3].map((i) => (
            <div className="person-row person-row--skeleton" key={i}>
              <span className="skeleton person-avatar person-avatar--md" />
              <span className="person-row__body">
                <span className="skeleton" />
                <span className="skeleton" />
              </span>
            </div>
          ))}
        </div>
      ) : resource.error ? (
        <ErrorState message={resource.error} onRetry={resource.retry} />
      ) : !resource.data?.length ? (
        <EmptyState
          title={query ? 'Никого не нашли' : text.emptyTitle}
          description={
            query
              ? 'Проверь написание или попробуй оригинальное имя латиницей.'
              : text.emptyDescription
          }
        />
      ) : (
        <ul className="person-list">
          {resource.data.map((person) => {
            const current = state(person)
            const busy = pending === person.tmdb_id
            return (
              <li key={person.tmdb_id} className="person-row">
                <button
                  type="button"
                  className="person-row__open"
                  disabled={busy}
                  onClick={() => open(person)}
                >
                  <PersonAvatar name={person.name} url={person.photo_url} />
                  <span className="person-row__body">
                    <strong>{person.name}</strong>
                    <span className="muted small">{personMeta(person)}</span>
                    {person.known_for.length > 0 && (
                      <span className="small person-row__known">
                        {person.known_for.join(' · ')}
                      </span>
                    )}
                  </span>
                </button>
                <button
                  type="button"
                  className={`follow-toggle ${current.followed ? 'follow-toggle--on' : ''}`}
                  aria-pressed={current.followed}
                  aria-busy={busy || undefined}
                  disabled={busy}
                  onClick={() => toggle(person)}
                >
                  {busy ? (
                    <span className="spinner" aria-hidden="true" />
                  ) : current.followed ? (
                    '✓'
                  ) : (
                    '＋'
                  )}
                  <span className="visually-hidden">
                    {current.followed
                      ? `Не отслеживать ${person.name}`
                      : `Отслеживать ${person.name}`}
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      )}
      <InlineError message={action.error} />
      {text.hint && resource.data?.length ? (
        <p className="person-hint muted small">
          {text.hint[0]}
          <br />
          {text.hint[1]}
        </p>
      ) : null}
    </Shell>
  )
}
