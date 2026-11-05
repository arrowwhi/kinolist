import test from 'node:test'
import assert from 'node:assert/strict'
import {
  candidateKey,
  contentCount,
  movieMetadata,
  seasonCount,
} from '../src/entities/movie/metadata.ts'
import { defaultFilters } from '../src/features/collection/model.ts'

test('film and series with the same external ID have distinct identities', () => {
  const film = {
    provider: 'tmdb',
    external_id: 7,
    media_type: 'movie' as const,
  }
  assert.notEqual(
    candidateKey(film),
    candidateKey({ ...film, media_type: 'tv' }),
  )
  assert.equal(
    candidateKey({ provider: 'tmdb', external_id: 7 }),
    candidateKey(film),
  )
})
test('Russian season labels include unknown and do not display zero seasons', () => {
  assert.deepEqual([1, 2, 5, 11, 21, 24].map(seasonCount), [
    '1 сезон',
    '2 сезона',
    '5 сезонов',
    '11 сезонов',
    '21 сезон',
    '24 сезона',
  ])
  for (const n of [undefined, null, 0, -1])
    assert.equal(seasonCount(n), 'Сезоны не указаны')
})
test('search can omit unloaded seasons, while details explicitly show unknown seasons', () => {
  const series = { media_type: 'tv' as const, year: 2022, season_count: null }
  assert.equal(movieMetadata(series, { search: true }), 'Сериал · 2022')
  assert.equal(movieMetadata(series), 'Сериал · 2022 · Сезоны не указаны')
  assert.equal(
    movieMetadata({ ...series, season_count: 2 }),
    'Сериал · 2022 · 2 сезона',
  )
  assert.equal(
    movieMetadata({ media_type: 'movie', year: 2024, season_count: 2 }),
    'Фильм · 2024',
  )
})
test('statistics labels match the current media filter and reset returns all types', () => {
  assert.equal(contentCount(21), '21 запись')
  assert.equal(contentCount(2, 'tv'), '2 сериала')
  assert.equal(contentCount(5, 'movie'), '5 фильмов')
  assert.equal(defaultFilters('watched').mediaType, undefined)
})
