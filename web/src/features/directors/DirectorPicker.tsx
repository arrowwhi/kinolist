import { useState } from 'react'
import type { DirectorSelection } from '../../api/types'
import { useDebounced } from '../../shared/hooks/useDebounced'
import { Field } from '../../shared/ui/Field'
import { Chip } from '../../shared/ui/Chip'
import { Button } from '../../shared/ui/Button'
import { ErrorState, SkeletonList } from '../../shared/ui/Feedback'
import { useDirectors } from './useDirectors'
export function DirectorPicker({
  value = [],
  onChange,
}: {
  value?: DirectorSelection[]
  onChange: (value: DirectorSelection[]) => void
}) {
  const [search, setSearch] = useState('')
  const query = useDebounced(search)
  const resource = useDirectors(
    { query, sort: 'name', order: 'asc', limit: 30 },
    0,
  )
  return (
    <fieldset className="stack stack--tight">
      <legend>Режиссёры</legend>
      {value.length > 0 && (
        <div className="chips chips--wrap">
          {value.map((d) => (
            <Chip
              key={d.id}
              active
              onClick={() => onChange(value.filter((x) => x.id !== d.id))}
              aria-label={`Убрать фильтр: ${d.name}`}
            >
              {d.name} ×
            </Chip>
          ))}
        </div>
      )}
      <Field
        type="search"
        placeholder="Найти режиссёра"
        aria-label="Найти режиссёра для фильтра"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <div className="director-picker__list" aria-label="Выбор режиссёров">
        {resource.loading ? (
          <SkeletonList />
        ) : resource.error && !resource.page ? (
          <ErrorState message={resource.error} onRetry={resource.retry} />
        ) : !resource.page?.items.length ? (
          <p className="muted small">
            {query
              ? 'Режиссёр не найден.'
              : 'В коллекции пока нет работ с указанными режиссёрами.'}
          </p>
        ) : (
          resource.page.items.map((d) => (
            <label key={d.id} className="director-picker__option">
              <input
                type="checkbox"
                checked={value.some((x) => x.id === d.id)}
                onChange={(e) =>
                  onChange(
                    e.target.checked
                      ? [...value, { id: d.id, name: d.name }]
                      : value.filter((x) => x.id !== d.id),
                  )
                }
              />
              <span>
                {d.name}
                {d.original_name && d.original_name !== d.name && (
                  <small className="muted">{d.original_name}</small>
                )}
              </span>
            </label>
          ))
        )}
      </div>
      {resource.error && resource.page && (
        <ErrorState message={resource.error} onRetry={resource.loadMore} />
      )}
      {resource.page && resource.page.items.length < resource.page.total && (
        <Button
          variant="secondary"
          onClick={resource.loadMore}
          busy={resource.loadingMore}
        >
          Ещё режиссёры
        </Button>
      )}
      <p className="muted small">
        Любой из выбранных режиссёров. Только работы из твоей коллекции.
      </p>
    </fieldset>
  )
}
