import { useEffect, useState } from 'react'
import { errorMessage } from '../lib/errors'
// A cancelled request cannot replace data belonging to the next page or filter.
export function useResource<T>(
  load: (signal: AbortSignal) => Promise<T>,
  key: string | number,
) {
  const [state, setState] = useState<{
    data: T | null
    error: string | null
    loading: boolean
  }>({ data: null, error: null, loading: true })
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    const controller = new AbortController()
    setState({ data: null, error: null, loading: true })
    load(controller.signal)
      .then((data) => {
        if (!controller.signal.aborted)
          setState({ data, error: null, loading: false })
      })
      .catch((error) => {
        if (!controller.signal.aborted)
          setState({ data: null, error: errorMessage(error), loading: false })
      })
    return () => controller.abort()
    // The caller supplies a complete request key; loader identity is intentionally ignored.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, attempt])
  return { ...state, retry: () => setAttempt((value) => value + 1) }
}
