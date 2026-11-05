import test from 'node:test'
import assert from 'node:assert/strict'
import { home, routeFromHash } from '../src/app/route.ts'

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
