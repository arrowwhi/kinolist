import type { ListParams } from '../../api/client'
import type { MediaType, Status, UserMovie } from '../../api/types'
export type CollectionStatus = Status | 'all'
export type GroupBy = 'date' | 'genre' | 'none'
export interface CollectionFilters {
  mediaType?: MediaType
  genreIds: number[]
  sort: NonNullable<ListParams['sort']>
  order: 'asc' | 'desc'
  groupBy: GroupBy
}
export function defaultFilters(status: CollectionStatus): CollectionFilters {
  return {
    mediaType: undefined,
    genreIds: [],
    sort: status === 'watched' ? 'watched_at' : 'added_at',
    order: 'desc',
    groupBy: 'date',
  }
}
export function groupMovies(
  items: UserMovie[],
  groupBy: GroupBy,
  status: CollectionStatus,
) {
  if (groupBy === 'none') return [{ title: '', items }]
  const groups = new Map<string, UserMovie[]>()
  for (const item of items) {
    const value = status === 'watched' ? item.watched_at : item.added_at
    const date = value ? new Date(value) : null
    const labels =
      groupBy === 'genre'
        ? item.movie.genres.length
          ? item.movie.genres.map((g) => g.name_ru)
          : ['Без жанра']
        : [
            date && !Number.isNaN(date.getTime())
              ? new Intl.DateTimeFormat('ru-RU', {
                  month: 'long',
                  year: 'numeric',
                }).format(date)
              : 'Без даты просмотра',
          ]
    for (const label of labels)
      groups.set(label, [...(groups.get(label) ?? []), item])
  }
  const result = Array.from(groups, ([title, movies]) => ({
    title,
    items: movies,
  }))
  return groupBy === 'genre'
    ? result.sort((a, b) => a.title.localeCompare(b.title, 'ru'))
    : result
}
