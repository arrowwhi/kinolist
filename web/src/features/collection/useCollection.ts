import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/client'
import type { MovieList } from '../../api/types'
import { errorMessage } from '../../shared/lib/errors'
import type { CollectionFilters, CollectionStatus } from './model'
export function useCollection(
  status: CollectionStatus,
  filters: CollectionFilters,
  query: string,
  revision: number,
  fixedGenre?: number,
) {
  const summaryKey = JSON.stringify([
    status,
    filters.genreIds,
    filters.mediaType,
    revision,
    fixedGenre,
  ])
  const [summaryState, setSummaryState] = useState<{
    key: string
    value: MovieList['summary']
  } | null>(null)
  const summary = summaryState?.key === summaryKey ? summaryState.value : null
  const [page, setPage] = useState<MovieList | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)
  const generation = useRef(0)
  const morePending = useRef(false)
  const key = JSON.stringify([
    status,
    filters.genreIds,
    filters.mediaType,
    filters.sort,
    filters.order,
    query,
    revision,
    fixedGenre,
    attempt,
  ])
  const params = {
    status: status === 'all' ? undefined : status,
    mediaType: filters.mediaType,
    genreIds: fixedGenre ? [fixedGenre] : filters.genreIds,
    query,
    sort: filters.sort,
    order: filters.order,
    limit: 30,
  }
  useEffect(() => {
    const current = ++generation.current
    const controller = new AbortController()
    morePending.current = false
    setPage(null)
    setLoading(true)
    setLoadingMore(false)
    setError(null)
    api
      .listMovies(params, controller.signal)
      .then((data) => {
        if (current === generation.current && !controller.signal.aborted) {
          setPage(data)
          setSummaryState({ key: summaryKey, value: data.summary })
        }
      })
      .catch((err) => {
        if (!controller.signal.aborted && current === generation.current)
          setError(errorMessage(err))
      })
      .finally(() => {
        if (!controller.signal.aborted && current === generation.current)
          setLoading(false)
      })
    return () => {
      controller.abort()
      generation.current++
    }
    // Request identity is represented by key.
  }, [key])
  async function loadMore() {
    if (!page || morePending.current || page.items.length >= page.total) return
    const current = generation.current
    morePending.current = true
    setLoadingMore(true)
    setError(null)
    try {
      const next = await api.listMovies({
        ...params,
        offset: page.items.length,
      })
      if (current === generation.current)
        setPage((previous) =>
          previous
            ? {
                ...next,
                items: [
                  ...previous.items,
                  ...next.items.filter(
                    (n) => !previous.items.some((p) => p.id === n.id),
                  ),
                ],
              }
            : next,
        )
    } catch (err) {
      if (current === generation.current) setError(errorMessage(err))
    } finally {
      if (current === generation.current) {
        morePending.current = false
        setLoadingMore(false)
      }
    }
  }
  return {
    page,
    summary,
    loading,
    loadingMore,
    error,
    loadMore,
    retry: () => setAttempt((v) => v + 1),
  }
}
