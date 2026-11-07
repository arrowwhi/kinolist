import type { DirectorListParams } from '../../api/client'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { Field } from '../../shared/ui/Field'
import { Chip } from '../../shared/ui/Chip'
import { Button } from '../../shared/ui/Button'
import { EmptyState, ErrorState, SkeletonList } from '../../shared/ui/Feedback'
import { DirectorCard } from '../../entities/director/DirectorCard'
import { directorCount } from '../../entities/director/metadata'
import { useDebounced } from '../../shared/hooks/useDebounced'
import { useDirectors } from '../../features/directors/useDirectors'
export type DirectorPreferences = Pick<
  DirectorListParams,
  'mediaType' | 'sort'
> & { search: string }
export function DirectorsPage({
  value,
  onChange,
  revision,
  navigate,
  back,
}: {
  value: DirectorPreferences
  onChange: (value: DirectorPreferences) => void
  revision: number
  navigate: Navigate
  back: () => void
}) {
  const query = useDebounced(value.search)
  const resource = useDirectors(
    {
      query,
      mediaType: value.mediaType,
      sort: value.sort ?? 'name',
      order: !value.sort || value.sort === 'name' ? 'asc' : 'desc',
      limit: 30,
    },
    revision,
  )
  return (
    <Shell
      title="Режиссёры"
      subtitle="Авторы историй в твоей коллекции."
      navigate={navigate}
      onBack={back}
    >
      <div className="chips" role="group" aria-label="Тип работ">
        {(
          [
            { key: undefined, label: 'Все' },
            { key: 'movie', label: 'Фильмы' },
            { key: 'tv', label: 'Сериалы' },
          ] as const
        ).map((o) => (
          <Chip
            key={o.label}
            active={value.mediaType === o.key}
            onClick={() => onChange({ ...value, mediaType: o.key })}
          >
            {o.label}
          </Chip>
        ))}
      </div>
      <Field
        type="search"
        placeholder="Поиск по имени режиссёра"
        aria-label="Поиск по имени режиссёра"
        value={value.search}
        onChange={(e) => onChange({ ...value, search: e.target.value })}
      />
      <div className="chips" role="group" aria-label="Сортировка режиссёров">
        {(
          [
            { key: 'name', label: 'По имени' },
            { key: 'work_count', label: 'По работам' },
            { key: 'average_user_rating', label: 'По моей оценке' },
          ] as const
        ).map((o) => (
          <Chip
            key={o.key}
            active={(value.sort ?? 'name') === o.key}
            onClick={() => onChange({ ...value, sort: o.key })}
          >
            {o.label}
          </Chip>
        ))}
      </div>
      {resource.page && (
        <p className="section-label" aria-live="polite">
          {directorCount(resource.page.total)} ·{' '}
          {value.mediaType === 'movie'
            ? 'Фильмы'
            : value.mediaType === 'tv'
              ? 'Сериалы'
              : 'Все типы'}
        </p>
      )}
      {resource.loading ? (
        <SkeletonList />
      ) : resource.error && !resource.page ? (
        <ErrorState message={resource.error} onRetry={resource.retry} />
      ) : !resource.page?.items.length ? (
        <EmptyState
          title={query ? 'Режиссёр не найден' : 'Здесь будут твои режиссёры'}
          description={
            query
              ? 'Попробуй русское или оригинальное имя.'
              : value.mediaType
                ? 'В этом типе контента пока нет работ с указанным режиссёром.'
                : 'Добавь фильм или сериал — его режиссёры появятся здесь.'
          }
          action={
            query || value.mediaType
              ? 'Сбросить поиск и тип'
              : 'Добавить в коллекцию'
          }
          onAction={() =>
            query || value.mediaType
              ? onChange({ ...value, search: '', mediaType: undefined })
              : navigate('/add')
          }
        />
      ) : (
        <div className="stack">
          {resource.page.items.map((d) => (
            <DirectorCard
              key={d.id}
              director={d}
              onOpen={() => navigate(`/directors/${d.id}?status=all`)}
            />
          ))}
        </div>
      )}
      {resource.error && resource.page && (
        <ErrorState message={resource.error} onRetry={resource.loadMore} />
      )}
      {resource.hasMore && (
        <Button
          variant="secondary"
          busy={resource.loadingMore}
          onClick={resource.loadMore}
        >
          Показать ещё
        </Button>
      )}
    </Shell>
  )
}
