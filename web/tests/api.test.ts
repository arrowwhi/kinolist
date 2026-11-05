import test from 'node:test'
import assert from 'node:assert/strict'
import { api } from '../src/api/client.ts'

test('API preserves media type in pagination, genre counts, details and confirmation', async (t) => {
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
    await api.listMovies({
      mediaType: 'tv',
      status: 'watched',
      genreIds: [18],
      query: 'Shared',
      limit: 30,
      offset: 30,
    })
    const params = new URL(calls[0].url, 'http://localhost').searchParams
    assert.equal(params.get('media_type'), 'tv')
    assert.equal(params.get('offset'), '30')
    assert.equal(params.get('genre_ids'), '18')
    await api.collectionGenres(undefined, 'tv')
    assert.equal(
      new URL(calls[1].url, 'http://localhost').searchParams.get('media_type'),
      'tv',
    )
    await api.candidateDetails(5, {
      provider: 'tmdb',
      external_id: 7,
      media_type: 'tv',
    })
    assert.equal(calls[2].url, '/api/v1/jobs/5/candidates/tmdb/tv/7')
    await api.confirmJob(5, 'tmdb', 7, {
      media_type: 'tv',
      status: 'watched',
      user_rating: 9,
      review: 'Good',
    })
    assert.deepEqual(JSON.parse(String(calls[3].init?.body)), {
      provider: 'tmdb',
      external_id: 7,
      media_type: 'tv',
      status: 'watched',
      user_rating: 9,
      review: 'Good',
    })
    await api.listMovies()
    assert.equal(
      new URL(calls[4].url, 'http://localhost').searchParams.has('media_type'),
      false,
    )
  } finally {
    if (descriptor) Object.defineProperty(globalThis, 'window', descriptor)
    else Reflect.deleteProperty(globalThis, 'window')
  }
})
