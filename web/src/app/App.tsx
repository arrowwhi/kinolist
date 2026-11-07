import { DirectorsPage } from '../pages/directors/DirectorsPage'
import type { DirectorPreferences } from '../pages/directors/DirectorsPage'
import { DirectorWorksPage } from '../pages/directors/DirectorWorksPage'
import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '../api/client'
import type { CollectionStatus } from '../features/collection/model'
import { HomePage } from '../pages/home/HomePage'
import { HomePageV2 } from '../pages/home/HomePageV2'
import { MyDirectorsPage } from '../pages/directors-v2/MyDirectorsPage'
import type { MyDirectorsPreferences } from '../pages/directors-v2/MyDirectorsPage'
import {
  DirectorProfilePage,
  defaultFilmography,
} from '../pages/directors-v2/DirectorProfilePage'
import type { FilmographyPreferences } from '../pages/directors-v2/DirectorProfilePage'
import { AddDirectorPage } from '../pages/directors-v2/AddDirectorPage'
import { home } from './route'
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
import { useRecentCandidates } from '../features/add-movie/useRecentCandidates'
import { defaultFilters, directorFilters } from '../features/collection/model'
export function App() {
  const { location, navigate, back } = useNavigation()
  const history = useRecentCandidates()
  const [directorPreferences, setDirectorPreferences] =
    useState<DirectorPreferences>({ search: '', sort: 'name' })
  const [myDirectors, setMyDirectors] = useState<MyDirectorsPreferences>({
    scope: 'all',
    sort: 'name',
    search: '',
  })
  const [filmography, setFilmography] = useState<
    Record<number, FilmographyPreferences>
  >({})
  const [revision, setRevision] = useState(0)
  const [toast, setToast] = useState('')
  const timer = useRef<number | undefined>(undefined)
  const [preferences, setPreferences] = useState<
    Record<string, CollectionPreferences>
  >({})
  const path = location.split('?')[0]
  const params = new URLSearchParams(location.split('?')[1])
  const goHome = useCallback(() => navigate(home, true), [navigate])
  useTelegramSetup()
  const catalog = useResource((signal) => api.genres(signal), 'genres')
  const changed = useCallback((message: string) => {
    setRevision((value) => value + 1)
    setToast(message)
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setToast(''), 3000)
  }, [])
  useEffect(() => () => window.clearTimeout(timer.current), [])
  const routeMatch = path.match(/^\/movie\/(\d+)(?:\/(note|rate))?$/)
  const directorMatch = path.match(/^\/directors\/(\d+)$/)
  const genreMatch = path.match(/^\/genres\/(\d+)$/)
  const directorV2Match = path.match(/^\/v2\/directors\/(\d+)$/)
  let content
  if (path === home)
    content = <HomePage navigate={navigate} revision={revision} />
  else if (path === '/v2')
    content = <HomePageV2 navigate={navigate} revision={revision} />
  else if (path === '/v2/directors')
    content = (
      <MyDirectorsPage
        value={myDirectors}
        onChange={setMyDirectors}
        revision={revision}
        navigate={navigate}
        back={goHome}
      />
    )
  else if (path === '/v2/directors/add')
    content = (
      <AddDirectorPage
        initialQuery={params.get('q') ?? ''}
        navigate={navigate}
        back={back}
        changed={changed}
      />
    )
  else if (directorV2Match && Number(directorV2Match[1]) > 0) {
    const id = Number(directorV2Match[1])
    content = (
      <DirectorProfilePage
        key={id}
        id={id}
        preferences={filmography[id] ?? defaultFilmography}
        onPreferences={(value) =>
          setFilmography((previous) => ({ ...previous, [id]: value }))
        }
        navigate={navigate}
        back={back}
        changed={changed}
      />
    )
  } else if (routeMatch && Number(routeMatch[1]) > 0) {
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
    content = (
      <AddMoviePage
        navigate={navigate}
        back={back}
        changed={changed}
        recent={history.recent}
        remember={history.remember}
      />
    )
  else if (path === '/genres')
    content = (
      <GenresPage navigate={navigate} revision={revision} back={goHome} />
    )
  else if (path === '/directors')
    content = (
      <DirectorsPage
        value={directorPreferences}
        onChange={setDirectorPreferences}
        revision={revision}
        navigate={navigate}
        back={goHome}
      />
    )
  else if (directorMatch && Number(directorMatch[1]) > 0) {
    const status: CollectionStatus =
      params.get('status') === 'watched'
        ? 'watched'
        : params.get('status') === 'want_to_watch'
          ? 'want_to_watch'
          : 'all'
    const key = `/directors/${directorMatch[1]}?status=${status}`
    content = (
      <DirectorWorksPage
        key={key}
        id={Number(directorMatch[1])}
        status={status}
        genres={catalog.data?.items ?? []}
        preferences={
          preferences[key] ?? {
            filters: directorFilters(status),
            search: '',
          }
        }
        onPreferences={(value) =>
          setPreferences((previous) => ({ ...previous, [key]: value }))
        }
        revision={revision}
        navigate={navigate}
        back={back}
      />
    )
  } else if (path === '/filters') {
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
    const defaults = returnPath.startsWith('/directors/')
      ? directorFilters(status)
      : defaultFilters(status)
    const current = preferences[returnTo] ?? { filters: defaults, search: '' }
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
        defaults={defaults}
        status={status}
        fixedGenre={fixedGenre}
        fixedDirectorId={
          returnPath.match(/^\/directors\/(\d+)$/)
            ? Number(returnPath.split('/')[2])
            : undefined
        }
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
          preferences={
            preferences[key] ?? { filters: defaultFilters(status), search: '' }
          }
          onPreferences={(value) =>
            setPreferences((previous) => ({ ...previous, [key]: value }))
          }
          revision={revision}
          navigate={navigate}
          back={genre ? back : goHome}
        />
      )
  } else
    content = (
      <Shell title="Страница не найдена" navigate={navigate} onBack={back}>
        <ErrorState message="Вернись к своей коллекции." onRetry={goHome} />
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
