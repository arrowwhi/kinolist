import { useEffect, useRef, useState } from 'react'
import { api } from '../../api/client'
import type { DirectorListParams } from '../../api/client'
import type { DirectorList } from '../../api/types'
import { errorMessage } from '../../shared/lib/errors'
export function useDirectors(params: DirectorListParams, revision: number) {
  const [page, setPage] = useState<DirectorList | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)
  const generation = useRef(0)
  const pending = useRef(false)
  const key = JSON.stringify([params, revision, attempt])
  useEffect(() => {
    const current = ++generation.current
    const controller = new AbortController()
    pending.current = false
    setPage(null)
    setLoading(true)
    setLoadingMore(false)
    setError(null)
    api
      .listDirectors(params, controller.signal)
      .then((data) => {
        if (current === generation.current && !controller.signal.aborted)
          setPage(data)
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
  }, [key])
  async function loadMore() {
    if (!page || pending.current || page.items.length >= page.total) return
    const current = generation.current
    pending.current = true
    setLoadingMore(true)
    setError(null)
    try {
      const next = await api.listDirectors({
        ...params,
        offset: page.offset + page.limit,
      })
      if (current === generation.current)
        setPage((previous) =>
          previous
            ? {
                ...next,
                items: [
                  ...previous.items,
                  ...next.items.filter(
                    (d) => !previous.items.some((p) => p.id === d.id),
                  ),
                ],
              }
            : next,
        )
    } catch (err) {
      if (current === generation.current) setError(errorMessage(err))
    } finally {
      if (current === generation.current) {
        pending.current = false
        setLoadingMore(false)
      }
    }
  }
  return {
    page,
    loading,
    loadingMore,
    error,
    loadMore,
    retry: () => setAttempt((v) => v + 1),
  }
}
