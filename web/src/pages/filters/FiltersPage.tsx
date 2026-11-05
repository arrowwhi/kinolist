import { useState } from 'react'
import type { Genre } from '../../api/types'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { Button } from '../../shared/ui/Button'
import { Chip } from '../../shared/ui/Chip'
import { defaultFilters } from '../../features/collection/model'
import type {
  CollectionFilters,
  CollectionStatus,
  GroupBy,
} from '../../features/collection/model'
export function FiltersPage({
  genres,
  status,
  value,
  onApply,
  navigate,
  back,
  fixedGenre,
}: {
  genres: Genre[]
  status: CollectionStatus
  value: CollectionFilters
  onApply: (filters: CollectionFilters) => void
  navigate: Navigate
  back: () => void
  fixedGenre?: Genre
}) {
  const [draft, setDraft] = useState(value)
  const options: { value: CollectionFilters['sort']; label: string }[] = [
    {
      value: status === 'watched' ? 'watched_at' : 'added_at',
      label: status === 'watched' ? 'По дате просмотра' : 'По дате добавления',
    },
    { value: 'title', label: 'По названию' },
    { value: 'year', label: 'По году выхода' },
    { value: 'rating', label: 'По рейтингу каталога' },
    ...(status === 'watched'
      ? [{ value: 'user_rating' as const, label: 'По моей оценке' }]
      : []),
  ]
  return (
    <Shell title="Фильтры" navigate={navigate} onBack={back}>
      <form
        className="stack filters-form"
        onSubmit={(e) => {
          e.preventDefault()
          onApply(draft)
        }}
      >
        <fieldset>
          <legend>Жанры</legend>
          {fixedGenre ? (
            <p className="muted">
              Жанр «{fixedGenre.name_ru}» выбран на странице коллекции.
            </p>
          ) : (
            <div className="chips chips--wrap">
              {genres.map((g) => (
                <Chip
                  key={g.id}
                  active={draft.genreIds.includes(g.id)}
                  onClick={() =>
                    setDraft({
                      ...draft,
                      genreIds: draft.genreIds.includes(g.id)
                        ? draft.genreIds.filter((id) => id !== g.id)
                        : [...draft.genreIds, g.id],
                    })
                  }
                >
                  {g.name_ru}
                </Chip>
              ))}
            </div>
          )}
        </fieldset>
        <fieldset>
          <legend>Сортировать</legend>
          <div className="stack stack--tight">
            {options.map((option) => (
              <label className="radio-row" key={option.value}>
                <input
                  type="radio"
                  name="sort"
                  checked={draft.sort === option.value}
                  onChange={() => setDraft({ ...draft, sort: option.value })}
                />
                <span>{option.label}</span>
              </label>
            ))}
          </div>
        </fieldset>
        <fieldset>
          <legend>Порядок</legend>
          <div className="chips">
            <Chip
              active={draft.order === 'desc'}
              onClick={() => setDraft({ ...draft, order: 'desc' })}
            >
              По убыванию
            </Chip>
            <Chip
              active={draft.order === 'asc'}
              onClick={() => setDraft({ ...draft, order: 'asc' })}
            >
              По возрастанию
            </Chip>
          </div>
        </fieldset>
        <fieldset>
          <legend>Группировать</legend>
          <div className="chips">
            {(
              [
                ['date', 'По дате'],
                ['genre', 'По жанру'],
                ['none', 'Без групп'],
              ] as [GroupBy, string][]
            ).map(([key, label]) => (
              <Chip
                key={key}
                active={draft.groupBy === key}
                onClick={() => setDraft({ ...draft, groupBy: key })}
              >
                {label}
              </Chip>
            ))}
          </div>
        </fieldset>
        <div className="form-actions">
          <Button type="submit">Применить фильтры</Button>
          <Button
            variant="secondary"
            onClick={() => setDraft(defaultFilters(status))}
          >
            Сбросить фильтры
          </Button>
        </div>
      </form>
    </Shell>
  )
}
