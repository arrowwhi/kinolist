import { useEffect, useState } from 'react'

// useDebounced не даёт слать запрос на каждое нажатие клавиши в поиске.
export function useDebounced<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])

  return debounced
}
