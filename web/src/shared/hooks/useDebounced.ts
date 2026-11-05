import { useEffect, useState } from 'react'
export function useDebounced<T>(value: T, delay = 250) {
  const [result, setResult] = useState(value)
  useEffect(() => {
    const timer = window.setTimeout(() => setResult(value), delay)
    return () => window.clearTimeout(timer)
  }, [value, delay])
  return result
}
