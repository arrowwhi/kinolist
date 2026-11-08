import test from 'node:test'
import assert from 'node:assert/strict'
import { home, routeFromHash } from '../src/app/route.ts'
import { catalogPath, parseCatalogRoute } from '../src/shared/lib/navigation.ts'

test('Telegram launch hash opens home instead of the not-found page', () => {
  assert.equal(
    routeFromHash(
      '#tgWebAppData=query_id%3Dexample&tgWebAppVersion=8.0&tgWebAppPlatform=ios',
    ),
    home,
  )
  assert.equal(routeFromHash('#tgWebAppThemeParams=%7B%7D'), home)
})

test('empty and root hashes open home', () => {
  for (const hash of ['', '#', '#/']) assert.equal(routeFromHash(hash), home)
})

test('app routes and query parameters survive parsing', () => {
  for (const route of [
    '/collection/watched',
    '/movie/12/note',
    '/genres/18?status=all',
  ])
    assert.equal(routeFromHash(`#${route}`), route)
})

test('unknown routes still reach the not-found page', () => {
  assert.equal(routeFromHash('#/missing'), '/missing')
  assert.equal(routeFromHash('#unknown'), 'unknown')
})

test('home is a dedicated route and collection deep links remain available', () => {
  assert.equal(home, '/')
  assert.equal(
    routeFromHash('#/collection/want_to_watch'),
    '/collection/want_to_watch',
  )
})

test('catalog card routes round-trip through the hash with the director hint', () => {
  const route = {
    provider: 'tmdb' as const,
    mediaType: 'movie' as const,
    externalId: 181886,
    directorId: 17,
  }
  const path = catalogPath(route)
  assert.equal(path, '/catalog/tmdb/movie/181886?director=17')
  assert.equal(routeFromHash(`#${path}`), path)
  assert.deepEqual(parseCatalogRoute(routeFromHash(`#${path}`)), route)
  assert.deepEqual(parseCatalogRoute('/catalog/kinopoisk/tv/5'), {
    provider: 'kinopoisk',
    mediaType: 'tv',
    externalId: 5,
  })
  assert.equal(
    catalogPath({ provider: 'tmdb', mediaType: 'tv', externalId: 3 }),
    '/catalog/tmdb/tv/3',
  )
})

test('malformed catalog routes are rejected', () => {
  for (const location of [
    '/catalog/tmdb/movie',
    '/catalog/imdb/movie/1',
    '/catalog/tmdb/anime/1',
    '/catalog/tmdb/movie/0',
    '/catalog/tmdb/movie/12/extra',
    '/movie/12',
  ])
    assert.equal(parseCatalogRoute(location), null, location)
  assert.equal(
    parseCatalogRoute('/catalog/tmdb/movie/1?director=abc')?.directorId,
    undefined,
  )
})
