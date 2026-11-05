import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import type { CollectionStatus } from '../features/collection/model'
import { useNavigation } from './navigation'
import { Shell } from '../widgets/app-shell/Shell'
import { useTelegramSetup } from '../shared/lib/telegram'
import { useResource } from '../shared/hooks/useResource'
import { ErrorState, SkeletonList } from '../shared/ui/Feedback'
import { CollectionPage } from '../pages/collection/CollectionPage'
import type { CollectionPreferences } from '../pages/collection/CollectionPage'
import { GenresPage } from '../pages/genres/GenresPage'
import { FiltersPage } from '../pages/filters/FiltersPage'
import { MoviePage } from '../pages/movie/MoviePage'
import { NotePage } from '../pages/movie/NotePage'
import { RatePage } from '../pages/movie/RatePage'
import { AddMoviePage } from '../pages/add-movie/AddMoviePage'
import { defaultFilters } from '../features/collection/model'
export function App() {
  const { location, navigate, back } = useNavigation()
  const [revision, setRevision] = useState(0)
  const [toast, setToast] = useState('')
  const timer = useRef<number | undefined>(undefined)
  const [preferences, setPreferences] = useState<
    Record<string, CollectionPreferences>
  >({})
  const path = location.split('?')[0]
  const params = new URLSearchParams(location.split('?')[1])
  const topLevel = [
    '/collection/want_to_watch',
    '/collection/watched',
    '/genres',
  ].includes(path)
  useTelegramSetup(back, !topLevel)
  const catalog = useResource((signal) => api.genres(signal), 'genres')
  const genreStats = useResource(
    (signal) => api.collectionGenres(signal),
    revision,
  )
  const changed = useCallback((message: string) => {
    setRevision((value) => value + 1)
    setToast(message)
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setToast(''), 3000)
  }, [])
  useEffect(() => () => window.clearTimeout(timer.current), [])
  const routeMatch = path.match(/^\/movie\/(\d+)(?:\/(note|rate))?$/)
  const genreMatch = path.match(/^\/genres\/(\d+)$/)
  let content
  if (routeMatch && Number(routeMatch[1]) > 0) {
    const props = { id: Number(routeMatch[1]), navigate, back, changed }
    content =
      routeMatch[2] === 'note' ? (
        <NotePage {...props} />
      ) : routeMatch[2] === 'rate' ? (
        <RatePage {...props} />
      ) : (
        <MoviePage {...props} revision={revision} />
      )
  } else if (path === '/add')
    content = <AddMoviePage navigate={navigate} back={back} changed={changed} />
  else if (path === '/genres')
    content = <GenresPage navigate={navigate} revision={revision} />
  else if (path === '/filters') {
    const returnTo = params.get('return') || '/collection/want_to_watch'
    const returnPath = returnTo.split('?')[0]
    const returnParams = new URLSearchParams(returnTo.split('?')[1])
    const status: CollectionStatus =
      returnPath.endsWith('/watched') ||
      returnParams.get('status') === 'watched'
        ? 'watched'
        : returnParams.get('status') === 'all'
          ? 'all'
          : 'want_to_watch'
    const current = preferences[returnTo] ?? {
      filters: defaultFilters(status),
      search: '',
    }
    const fixedGenre = catalog.data?.items.find(
      (g) => returnPath === `/genres/${g.id}`,
    )
    content = catalog.loading ? (
      <Shell title="Фильтры" navigate={navigate} onBack={back}>
        <SkeletonList />
      </Shell>
    ) : catalog.error ? (
      <Shell title="Фильтры" navigate={navigate} onBack={back}>
        <ErrorState message={catalog.error} onRetry={catalog.retry} />
      </Shell>
    ) : (
      <FiltersPage
        key={returnTo}
        genres={catalog.data?.items ?? []}
        value={current.filters}
        status={status}
        fixedGenre={fixedGenre}
        navigate={navigate}
        back={back}
        onApply={(filters) => {
          setPreferences((previous) => ({
            ...previous,
            [returnTo]: { ...current, filters },
          }))
          back()
        }}
      />
    )
  } else if (path.startsWith('/collection/') || genreMatch) {
    const status: CollectionStatus =
      path === '/collection/watched' || params.get('status') === 'watched'
        ? 'watched'
        : params.get('status') === 'all' && genreMatch
          ? 'all'
          : 'want_to_watch'
    const genre = catalog.data?.items.find(
      (g) => g.id === Number(genreMatch?.[1]),
    )
    const key = genre
      ? `/genres/${genre.id}?status=${status}`
      : `/collection/${status}`
    if (genreMatch && !genre)
      content = (
        <Shell title="Жанр" navigate={navigate} onBack={back}>
          {catalog.loading ? (
            <SkeletonList />
          ) : (
            <ErrorState
              message={catalog.error || 'Такой жанр не найден'}
              onRetry={catalog.error ? catalog.retry : back}
            />
          )}
        </Shell>
      )
    else
      content = (
        <CollectionPage
          key={key}
          status={status}
          genre={genre}
          genres={catalog.data?.items ?? []}
          genreStats={genreStats.data?.items ?? []}
          preferences={
            preferences[key] ?? { filters: defaultFilters(status), search: '' }
          }
          onPreferences={(value) =>
            setPreferences((previous) => ({ ...previous, [key]: value }))
          }
          revision={revision}
          navigate={navigate}
          back={back}
        />
      )
  } else
    content = (
      <Shell title="Страница не найдена" navigate={navigate} onBack={back}>
        <ErrorState
          message="Вернись к своей коллекции фильмов."
          onRetry={() => navigate('/collection/want_to_watch', true)}
        />
      </Shell>
    )
  return (
    <>
      {content}
      {toast && (
        <div role="status" className="toast" key={toast}>
          {toast}
        </div>
      )}
    </>
  )
}
