// Фильтры и сортировка фильмографии в профиле человека — на клиенте.
import type { FilmographyItem } from '../../api/people'

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

export function filterWorks<T extends FilmographyItem>(
  items: T[],
  filter: FilmographyFilter,
) {
  return items.filter((w) =>
    filter === 'all'
      ? true
      : filter === 'missing'
        ? !w.status
        : w.status === filter,
  )
}

// Работы без значения уходят в конец при любом направлении сортировки.
export function sortWorks<T extends FilmographyItem>(
  items: T[],
  prefs: Pick<FilmographyPreferences, 'sort' | 'descending'>,
) {
  const value = (item: T) =>
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

export function decade(year?: number) {
  return year ? `${Math.floor(year / 10) * 10}-е` : 'Без года'
}
