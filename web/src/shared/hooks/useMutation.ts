import { useEffect, useRef, useState } from 'react'
import { errorMessage } from '../lib/errors'
import { haptic } from '../lib/telegram'
export function useMutation() {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const pending = useRef(false)
  const alive = useRef(true)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])
  async function execute<T>(
    task: () => Promise<T>,
    onSuccess: (value: T) => void,
  ) {
    if (pending.current) return
    pending.current = true
    setBusy(true)
    setError(null)
    try {
      const result = await task()
      if (alive.current) {
        haptic('success')
        onSuccess(result)
      }
    } catch (err) {
      if (alive.current) {
        haptic('error')
        setError(errorMessage(err, 'Не удалось сохранить. Попробуйте ещё раз.'))
      }
    } finally {
      pending.current = false
      if (alive.current) setBusy(false)
    }
  }
  return { busy, error, execute }
}
