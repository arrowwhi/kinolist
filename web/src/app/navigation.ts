import { useCallback, useEffect, useState } from 'react'
import type { Navigate } from '../shared/lib/navigation'
const home = '/collection/want_to_watch'
export function useNavigation() {
  const [location, setLocation] = useState(
    () => window.location.hash.slice(1) || home,
  )
  useEffect(() => {
    const update = () => setLocation(window.location.hash.slice(1) || home)
    window.addEventListener('popstate', update)
    window.addEventListener('hashchange', update)
    return () => {
      window.removeEventListener('popstate', update)
      window.removeEventListener('hashchange', update)
    }
  }, [])
  const navigate: Navigate = useCallback((path, replace = false) => {
    const depth = Number(window.history.state?.kinolistDepth ?? 0)
    const state = { kinolistDepth: replace ? depth : depth + 1 }
    window.history[replace ? 'replaceState' : 'pushState'](
      state,
      '',
      `#${path}`,
    )
    setLocation(path)
    window.scrollTo(0, 0)
  }, [])
  const back = useCallback(() => {
    if (Number(window.history.state?.kinolistDepth ?? 0) > 0)
      window.history.back()
    else navigate(home, true)
  }, [navigate])
  return { location, navigate, back }
}
