import test from 'node:test'
import assert from 'node:assert/strict'
import {
  directorCount,
  personalAverage,
  ratedWorks,
  workCount,
} from '../src/entities/director/metadata.ts'
import { api } from '../src/api/client.ts'

test('director and rated work labels handle Russian plurals and missing ratings', () => {
  assert.deepEqual([1, 2, 5, 11, 21].map(directorCount), [
    '1 режиссёр',
    '2 режиссёра',
    '5 режиссёров',
    '11 режиссёров',
    '21 режиссёр',
  ])
  assert.deepEqual([0, 1, 3, 11].map(workCount), [
    '0 работ',
    '1 работа',
    '3 работы',
    '11 работ',
  ])
  assert.equal(ratedWorks(1), 'Оценена 1 просмотренная работа')
  assert.equal(ratedWorks(3), 'Оценены 3 просмотренные работы')
  assert.equal(ratedWorks(11), 'Оценены 11 просмотренных работ')
  assert.equal(personalAverage(null), '—')
  assert.equal(personalAverage(8.7), '8,7')
})

test('director API preserves collection scope, search, sorting and pagination', async (t) => {
  const calls: { url: string; init?: RequestInit }[] = []
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'window')
  Object.defineProperty(globalThis, 'window', { configurable: true, value: {} })
  t.mock.method(
    globalThis,
    'fetch',
    async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ url: String(input), init })
      return new Response('{}', {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    },
  )
  try {
    const controller = new AbortController()
    await api.listDirectors(
      {
        query: 'Denis',
        mediaType: 'tv',
        sort: 'average_user_rating',
        order: 'desc',
        limit: 30,
        offset: 30,
      },
      controller.signal,
    )
    const query = new URL(calls[0].url, 'http://localhost').searchParams
    assert.equal(query.get('q'), 'Denis')
    assert.equal(query.get('media_type'), 'tv')
    assert.equal(query.get('sort'), 'average_user_rating')
    assert.equal(query.get('order'), 'desc')
    assert.equal(query.get('offset'), '30')
    assert.equal(calls[0].init?.signal, controller.signal)
    await api.getDirector(17)
    assert.equal(calls[1].url, '/api/v1/directors/17')
    await api.listMovies({
      directorIds: [17, 25],
      mediaType: 'movie',
      genreIds: [18],
      status: 'watched',
      query: 'Дюна',
      offset: 30,
    })
    const works = new URL(calls[2].url, 'http://localhost').searchParams
    assert.equal(works.get('director_ids'), '17,25')
    assert.equal(works.get('genre_ids'), '18')
    assert.equal(works.get('media_type'), 'movie')
    assert.equal(works.get('status'), 'watched')
    assert.equal(works.get('offset'), '30')
    await api.collectionGenres(controller.signal, 'tv', [17])
    const genres = new URL(calls[3].url, 'http://localhost').searchParams
    assert.equal(genres.get('scope'), 'collection')
    assert.equal(genres.get('director_ids'), '17')
    assert.equal(genres.get('media_type'), 'tv')
  } finally {
    if (descriptor) Object.defineProperty(globalThis, 'window', descriptor)
    else Reflect.deleteProperty(globalThis, 'window')
  }
})
