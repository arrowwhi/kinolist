import test from 'node:test'
import assert from 'node:assert/strict'
import {
  defaultFilters,
  groupMovies,
} from '../src/features/collection/model.ts'
import { contentCount } from '../src/entities/movie/metadata.ts'
import {
  formatRuntime,
  safeExternalUrl,
  shortUrl,
} from '../src/shared/lib/format.ts'
import type { UserMovie } from '../src/api/types.ts'
const item = (id: number, added: string, watched?: string): UserMovie => ({
  id,
  added_at: added,
  watched_at: watched,
  status: watched ? 'watched' : 'want_to_watch',
  source: 'manual',
  review: '',
  movie: {
    id,
    title: `Film ${id}`,
    overview: '',
    genres: [{ id: 18, name_ru: 'Драма', name_en: 'Drama' }],
    rating_vote_count: null,
  },
})
test('watched grouping uses watched date, not added date', () => {
  const data = [
    item(1, '2025-01-01', '2026-11-05'),
    item(2, '2025-04-01', '2026-11-06'),
  ]
  const groups = groupMovies(data, 'date', 'watched')
  assert.equal(groups.length, 1)
  assert.match(groups[0].title, /ноябрь.*2026/)
  assert.equal(groups[0].items.length, 2)
})
test('watchlist grouping uses added date', () => {
  assert.equal(
    groupMovies(
      [item(1, '2026-01-01'), item(2, '2026-02-01')],
      'date',
      'want_to_watch',
    ).length,
    2,
  )
})
test('genre grouping preserves a multi-genre film in both groups', () => {
  const movie = item(1, '2026-01-01')
  movie.movie.genres.push({ id: 28, name_ru: 'Боевик', name_en: 'Action' })
  const groups = groupMovies([movie], 'genre', 'want_to_watch')
  assert.deepEqual(
    groups.map((g) => g.title),
    ['Боевик', 'Драма'],
  )
  assert.equal(groups[0].items[0].id, groups[1].items[0].id)
})
test('unrated watched entries with missing date do not produce an invalid heading', () => {
  assert.equal(
    groupMovies([item(1, '2026-01-01')], 'date', 'watched')[0].title,
    'Без даты просмотра',
  )
})
test('date grouping is dropped when the list is sorted by something else', () => {
  const data = [
    item(1, '2025-01-01', '2026-03-05'),
    item(2, '2025-04-01', '2026-01-06'),
  ]
  const groups = groupMovies(data, 'date', 'watched', 'user_rating')
  assert.equal(groups.length, 1)
  assert.equal(groups[0].title, '')
  assert.deepEqual(
    groups[0].items.map((i) => i.id),
    [1, 2],
  )
  assert.equal(groupMovies(data, 'date', 'watched', 'watched_at').length, 2)
})
test('default sort follows the date belonging to the current status', () => {
  assert.equal(defaultFilters('watched').sort, 'watched_at')
  assert.equal(defaultFilters('want_to_watch').sort, 'added_at')
})
test('Russian movie count labels', () => {
  assert.deepEqual(
    [1, 2, 5, 11, 21, 24].map((n) => contentCount(n, 'movie')),
    ['1 фильм', '2 фильма', '5 фильмов', '11 фильмов', '21 фильм', '24 фильма'],
  )
})
test('runtime is formatted without invented unknown values', () => {
  assert.equal(formatRuntime(undefined), '')
  assert.equal(formatRuntime(166), '2 ч 46 мин')
  assert.equal(formatRuntime(120), '2 ч')
})
test('external source links reject executable URL schemes', () => {
  assert.equal(safeExternalUrl('javascript:alert(1)'), null)
  assert.equal(safeExternalUrl('data:text/html,hello'), null)
  assert.equal(
    safeExternalUrl('https://www.instagram.com/reel/example/'),
    'https://www.instagram.com/reel/example/',
  )
})
test('source links are shown without scheme, www and trailing slash', () => {
  assert.equal(
    shortUrl('https://www.instagram.com/reel/C8xQp2LsK1a/'),
    'instagram.com/reel/C8xQp2LsK1a',
  )
  assert.equal(shortUrl('http://instagr.am/p/abc'), 'instagr.am/p/abc')
})
