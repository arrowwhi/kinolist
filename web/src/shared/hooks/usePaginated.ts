import { useEffect, useRef, useState } from 'react'
import { errorMessage } from '../lib/errors'
interface Paginated<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}
// Список с догрузкой «Показать ещё». Ответ, пришедший после смены фильтров,
// не может подмешаться в новый список: каждая выборка помечена поколением.
// Следующая порция берётся со смещения offset + limit прошлого ответа, а не
// по числу элементов: дедупликация на клиенте не должна сдвигать страницы.
export function usePaginated<T extends { id: number }, P extends Paginated<T>>(
  load: (offset: number, signal?: AbortSignal) => Promise<P>,
  key: string,
  onFirstPage?: (page: P) => void,
) {
  const [page, setPage] = useState<P | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)
  const generation = useRef(0)
  const morePending = useRef(false)
  useEffect(() => {
    const current = ++generation.current
    const controller = new AbortController()
    morePending.current = false
    setPage(null)
    setLoading(true)
    setLoadingMore(false)
    setError(null)
    load(0, controller.signal)
      .then((data) => {
        if (current === generation.current && !controller.signal.aborted) {
          setPage(data)
          onFirstPage?.(data)
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
    // Вызывающий передаёт полный ключ запроса; сама функция загрузки не в счёт.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, attempt])
  async function loadMore() {
    if (!page || morePending.current) return
    const offset = page.offset + page.limit
    if (offset >= page.total) return
    const current = generation.current
    morePending.current = true
    setLoadingMore(true)
    setError(null)
    try {
      const next = await load(offset)
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
    hasMore: !!page && page.offset + page.limit < page.total,
    loading,
    loadingMore,
    error,
    loadMore,
    retry: () => setAttempt((v) => v + 1),
  }
}
