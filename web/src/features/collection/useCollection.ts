import { useState } from 'react'
import { api } from '../../api/client'
import type { MovieList } from '../../api/types'
import { usePaginated } from '../../shared/hooks/usePaginated'
import type { CollectionFilters, CollectionStatus } from './model'
export function useCollection(
  status: CollectionStatus,
  filters: CollectionFilters,
  query: string,
  revision: number,
  fixedGenre?: number,
  fixedDirector?: number,
) {
  const summaryKey = JSON.stringify([
    status,
    filters.genreIds,
    filters.mediaType,
    revision,
    fixedGenre,
    fixedDirector,
    filters.directors,
  ])
  const [summaryState, setSummaryState] = useState<{
    key: string
    value: MovieList['summary']
  } | null>(null)
  const summary = summaryState?.key === summaryKey ? summaryState.value : null
  const key = JSON.stringify([
    status,
    filters.genreIds,
    filters.mediaType,
    filters.sort,
    filters.order,
    query,
    revision,
    fixedGenre,
    fixedDirector,
    filters.directors,
  ])
  const params = {
    status: status === 'all' ? undefined : status,
    mediaType: filters.mediaType,
    genreIds: fixedGenre ? [fixedGenre] : filters.genreIds,
    query,
    directorIds: fixedDirector
      ? [fixedDirector]
      : filters.directors?.map((d) => d.id),
    sort: filters.sort,
    order: filters.order,
    limit: 30,
  }
  const list = usePaginated(
    (offset, signal) => api.listMovies({ ...params, offset }, signal),
    key,
    (data) => setSummaryState({ key: summaryKey, value: data.summary }),
  )
  return { ...list, summary }
}
