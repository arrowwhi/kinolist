import { directorsApi } from '../../api/directors'
import type { MyDirectorsScope, MyDirectorsSort } from '../../api/directors'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { Field } from '../../shared/ui/Field'
import { Chip } from '../../shared/ui/Chip'
import { Button } from '../../shared/ui/Button'
import { EmptyState, ErrorState, SkeletonList } from '../../shared/ui/Feedback'
import { useResource } from '../../shared/hooks/useResource'
import { useDebounced } from '../../shared/hooks/useDebounced'
import { DirectorOverviewCard } from '../../entities/director-v2/DirectorOverviewCard'
import { directorCount } from '../../entities/director/metadata'

export type { MyDirectorsScope, MyDirectorsSort }
export interface MyDirectorsPreferences {
  scope: MyDirectorsScope
  sort: MyDirectorsSort
  search: string
}

export function MyDirectorsPage({
  value,
  onChange,
  revision,
  navigate,
  back,
}: {
  value: MyDirectorsPreferences
  onChange: (value: MyDirectorsPreferences) => void
  revision: number
  navigate: Navigate
  back: () => void
}) {
  const query = useDebounced(value.search).trim()
  const resource = useResource(
    (signal) =>
      directorsApi.myDirectors(
        { scope: value.scope, sort: value.sort, query },
        signal,
      ),
    JSON.stringify([value.scope, value.sort, query, revision]),
  )
  const counts = resource.data?.counts
  const visible = resource.data?.items ?? []
  return (
    <Shell
      title="Режиссёры"
      subtitle="Следи за любимыми авторами и смотри их фильмографию целиком."
      navigate={navigate}
      onBack={back}
      floating={
        <Button
          className="button--pill"
          onClick={() => navigate('/v2/directors/add')}
        >
          ＋ Добавить режиссёра
        </Button>
      }
    >
      <div className="chips" role="group" aria-label="Какие режиссёры">
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
        placeholder="Поиск среди моих режиссёров"
        aria-label="Поиск среди моих режиссёров"
        value={value.search}
        onChange={(e) => onChange({ ...value, search: e.target.value })}
      />
      <div className="chips" role="group" aria-label="Сортировка режиссёров">
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
        <p className="muted small">
          Режиссёры фильмов из твоей коллекции. Отметь «Отслеживать» в профиле,
          чтобы закрепить автора.
        </p>
      )}
      {resource.loading ? (
        <SkeletonList />
      ) : resource.error ? (
        <ErrorState message={resource.error} onRetry={resource.retry} />
      ) : !visible.length ? (
        <EmptyState
          title={
            query
              ? 'Режиссёр не найден'
              : value.scope === 'followed'
                ? 'Пока никого не отслеживаешь'
                : 'Здесь будут твои режиссёры'
          }
          description={
            query
              ? 'Попробуй русское или оригинальное имя — или найди его в каталоге.'
              : 'Добавь режиссёра, чтобы видеть всю его фильмографию и что уже посмотрено.'
          }
          action={query ? 'Искать в каталоге' : 'Добавить режиссёра'}
          onAction={() =>
            navigate(
              query
                ? `/v2/directors/add?q=${encodeURIComponent(query)}`
                : '/v2/directors/add',
            )
          }
        />
      ) : (
        <>
          <p className="section-label" aria-live="polite">
            {directorCount(resource.data?.total ?? visible.length)}
          </p>
          <div className="stack">
            {visible.map((d) => (
              <DirectorOverviewCard
                key={d.id}
                director={d}
                onOpen={() => navigate(`/v2/directors/${d.id}`)}
              />
            ))}
          </div>
        </>
      )}
    </Shell>
  )
}
