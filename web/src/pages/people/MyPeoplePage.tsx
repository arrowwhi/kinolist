import type { MyPeopleScope, MyPeopleSort } from '../../api/people'
import { addPersonPath, personPath } from '../../shared/lib/navigation'
import type { Navigate } from '../../shared/lib/navigation'
import type { PeopleSection } from '../../features/people/sections'
import { Shell } from '../../widgets/app-shell/Shell'
import { Field } from '../../shared/ui/Field'
import { Chip } from '../../shared/ui/Chip'
import { Button } from '../../shared/ui/Button'
import { EmptyState, ErrorState, SkeletonList } from '../../shared/ui/Feedback'
import { useResource } from '../../shared/hooks/useResource'
import { useDebounced } from '../../shared/hooks/useDebounced'
import { PersonOverviewCard } from '../../entities/person/PersonOverviewCard'

export interface MyPeoplePreferences {
  scope: MyPeopleScope
  sort: MyPeopleSort
  search: string
}
export const defaultMyPeople: MyPeoplePreferences = {
  scope: 'all',
  sort: 'name',
  search: '',
}

// Экраны «22 · Мои режиссёры» и «30 · Мои актёры».
export function MyPeoplePage({
  section,
  value,
  onChange,
  revision,
  navigate,
  back,
}: {
  section: PeopleSection
  value: MyPeoplePreferences
  onChange: (value: MyPeoplePreferences) => void
  revision: number
  navigate: Navigate
  back: () => void
}) {
  const text = section.list
  const query = useDebounced(value.search).trim()
  const resource = useResource(
    (signal) =>
      section.api.myPeople(
        { scope: value.scope, sort: value.sort, query },
        signal,
      ),
    JSON.stringify([section.kind, value.scope, value.sort, query, revision]),
  )
  const counts = resource.data?.counts
  const visible = resource.data?.items ?? []
  return (
    <Shell
      title={text.title}
      subtitle={text.subtitle}
      navigate={navigate}
      onBack={back}
      floating={
        <Button
          className="button--pill"
          onClick={() => navigate(addPersonPath(section.kind))}
        >
          {text.add}
        </Button>
      }
    >
      <div className="chips" role="group" aria-label={text.scopeLabel}>
        {(
          [
            { key: 'all', label: 'Все', count: counts?.all },
            { key: 'followed', label: 'Отслеживаю', count: counts?.followed },
            {
              key: 'collection',
              label: 'Из коллекции',
              count: counts?.collection,
            },
          ] as const
        ).map((o) => (
          <Chip
            key={o.key}
            active={value.scope === o.key}
            onClick={() => onChange({ ...value, scope: o.key })}
          >
            {o.label}
            {o.count != null && <span className="chip__count">{o.count}</span>}
          </Chip>
        ))}
      </div>
      <Field
        type="search"
        placeholder={text.searchPlaceholder}
        aria-label={text.searchPlaceholder}
        value={value.search}
        onChange={(e) => onChange({ ...value, search: e.target.value })}
      />
      <div className="chips" role="group" aria-label={text.sortLabel}>
        {(
          [
            { key: 'name', label: 'По имени' },
            { key: 'progress', label: 'По просмотренному' },
            { key: 'user_rating', label: 'По моей оценке' },
          ] as const
        ).map((o) => (
          <Chip
            key={o.key}
            active={value.sort === o.key}
            onClick={() => onChange({ ...value, sort: o.key })}
          >
            {o.label}
          </Chip>
        ))}
      </div>
      {value.scope === 'collection' && resource.data && (
        <p className="muted small">{text.collectionHint}</p>
      )}
      {resource.loading ? (
        <SkeletonList />
      ) : resource.error ? (
        <ErrorState message={resource.error} onRetry={resource.retry} />
      ) : !visible.length ? (
        <EmptyState
          title={
            query
              ? text.notFound
              : value.scope === 'followed'
                ? text.emptyFollowed
                : text.empty
          }
          description={
            query
              ? 'Попробуй русское или оригинальное имя — или найди его в каталоге.'
              : text.emptyDescription
          }
          action={query ? 'Искать в каталоге' : text.addAction}
          onAction={() => navigate(addPersonPath(section.kind, query))}
        />
      ) : (
        <>
          <p className="section-label" aria-live="polite">
            {text.count(resource.data?.total ?? visible.length)}
          </p>
          <div className="stack">
            {visible.map((person) => (
              <PersonOverviewCard
                key={person.id}
                person={person}
                onOpen={() => navigate(personPath(section.kind, person.id))}
              />
            ))}
          </div>
        </>
      )}
    </Shell>
  )
}
