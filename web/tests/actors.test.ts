import test from 'node:test'
import assert from 'node:assert/strict'
import {
  actorCount,
  castKey,
  castTarget,
  roleCount,
  roleMeta,
} from '../src/entities/actor/metadata.ts'
import { workMeta } from '../src/entities/person/metadata.ts'
import {
  addPersonPath,
  catalogPath,
  parseCatalogRoute,
  parsePeopleRoute,
  personPath,
} from '../src/shared/lib/navigation.ts'
import {
  decade,
  filterWorks,
  sortWorks,
} from '../src/features/people/filmography.ts'
import type { FilmographyItem } from '../src/api/people.ts'
import { actorsApi } from '../src/api/actors.ts'
import { directorsApi } from '../src/api/directors.ts'

const work = (patch: Partial<FilmographyItem>): FilmographyItem => ({
  tmdb_id: 1,
  media_type: 'movie',
  title: 'Дюна: Часть вторая',
  original_title: 'Dune: Part Two',
  year: 2024,
  rating: 8.1,
  vote_count: 100,
  upcoming: false,
  ...patch,
})

test('actor and role counters use Russian plurals', () => {
  assert.deepEqual([1, 2, 5, 11, 21, 22].map(actorCount), [
    '1 актёр',
    '2 актёра',
    '5 актёров',
    '11 актёров',
    '21 актёр',
    '22 актёра',
  ])
  assert.deepEqual([0, 1, 3, 12, 101].map(roleCount), [
    '0 ролей',
    '1 роль',
    '3 роли',
    '12 ролей',
    '101 роль',
  ])
})

test('role row shows year, media type and character instead of the original title', () => {
  assert.equal(
    roleMeta({ ...work({}), character: 'Пол Атрейдес' }),
    '2024 · Фильм · роль: Пол Атрейдес',
  )
  assert.equal(
    roleMeta({
      ...work({ media_type: 'tv', episode_count: 19, year: undefined }),
      character: 'Марк Скаут',
    }),
    'Год неизвестен · Сериал · 19 эп. · роль: Марк Скаут',
  )
  // Персонаж неизвестен или его нет в старом ответе — без «роль:».
  assert.equal(roleMeta({ ...work({}), character: '  ' }), '2024 · Фильм')
  assert.equal(roleMeta(work({})), '2024 · Фильм')
  // У режиссёра строка прежняя — с оригинальным названием.
  assert.equal(workMeta(work({})), '2024 · Фильм · Dune: Part Two')
  assert.equal(
    workMeta(work({ original_title: 'Дюна: Часть вторая' })),
    '2024 · Фильм',
  )
})

test('cast member opens by internal id, by TMDB import, or not at all', () => {
  const base = {
    name: 'Адам Скотт',
    original_name: 'Adam Scott',
    character: '',
  }
  assert.deepEqual(castTarget({ ...base, id: 7, tmdb_id: 55 }), { id: 7 })
  assert.deepEqual(castTarget({ ...base, tmdb_id: 55 }), { tmdbId: 55 })
  assert.equal(castTarget({ ...base, kinopoisk_id: 9 }), null)
  assert.equal(castKey({ ...base, id: 7, tmdb_id: 55 }, 0), 'id:7')
  assert.equal(castKey({ ...base, tmdb_id: 55 }, 1), 'tmdb:55')
  assert.equal(castKey(base, 3), 'i:3')
})

test('people routes resolve list, search and profile for both sections', () => {
  assert.deepEqual(parsePeopleRoute('/actors'), { kind: 'actor', page: 'list' })
  assert.deepEqual(parsePeopleRoute('/actors/add'), {
    kind: 'actor',
    page: 'add',
  })
  assert.deepEqual(parsePeopleRoute('/actors/12'), {
    kind: 'actor',
    page: 'profile',
    id: 12,
  })
  assert.deepEqual(parsePeopleRoute('/v2/directors/3'), {
    kind: 'director',
    page: 'profile',
    id: 3,
  })
  assert.deepEqual(parsePeopleRoute('/v2/directors'), {
    kind: 'director',
    page: 'list',
  })
  for (const path of [
    '/actors/0',
    '/actors/x',
    '/actors/12/roles',
    '/directors/3',
  ])
    assert.equal(parsePeopleRoute(path), null)
  assert.equal(personPath('actor', 12), '/actors/12')
  assert.equal(personPath('director', 3), '/v2/directors/3')
  assert.equal(addPersonPath('actor'), '/actors/add')
  assert.equal(
    addPersonPath('actor', 'Том Харди'),
    '/actors/add?q=%D0%A2%D0%BE%D0%BC%20%D0%A5%D0%B0%D1%80%D0%B4%D0%B8',
  )
})

test('catalog card remembers the actor profile it was opened from', () => {
  const route = {
    provider: 'tmdb' as const,
    mediaType: 'movie' as const,
    externalId: 693134,
    actorId: 12,
  }
  assert.equal(catalogPath(route), '/catalog/tmdb/movie/693134?actor=12')
  assert.deepEqual(parseCatalogRoute(catalogPath(route)), route)
  assert.deepEqual(parseCatalogRoute('/catalog/tmdb/tv/1?actor=-1'), {
    provider: 'tmdb',
    mediaType: 'tv',
    externalId: 1,
  })
  assert.equal(
    catalogPath({ ...route, actorId: undefined, directorId: 4 }),
    '/catalog/tmdb/movie/693134?director=4',
  )
})

test('filmography filters, sorts with blanks last and groups by decade', () => {
  const items = [
    work({ tmdb_id: 1, year: 2017, rating: 7.3, status: 'want_to_watch' }),
    work({ tmdb_id: 2, year: 2024, rating: null, status: 'watched' }),
    work({ tmdb_id: 3, year: 2019, rating: 7.8 }),
  ]
  assert.deepEqual(
    filterWorks(items, 'missing').map((w) => w.tmdb_id),
    [3],
  )
  assert.deepEqual(
    sortWorks(items, { sort: 'year', descending: true }).map((w) => w.year),
    [2024, 2019, 2017],
  )
  assert.deepEqual(
    sortWorks(items, { sort: 'rating', descending: false }).map(
      (w) => w.tmdb_id,
    ),
    [1, 3, 2],
  )
  assert.equal(decade(2019), '2010-е')
  assert.equal(decade(undefined), 'Без года')
})

test('actors API uses the actor endpoints and role=actor search', async (t) => {
  const calls: { url: string; init?: RequestInit }[] = []
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, 'window')
  Object.defineProperty(globalThis, 'window', { configurable: true, value: {} })
  t.mock.method(
    globalThis,
    'fetch',
    async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ url: String(input), init })
      return init?.method === 'PUT' || init?.method === 'DELETE'
        ? new Response(null, { status: 204 })
        : new Response('{"items":[]}', {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          })
    },
  )
  try {
    await actorsApi.myPeople({
      scope: 'collection',
      sort: 'progress',
      query: 'Шаламе',
    })
    const list = new URL(calls[0].url, 'http://localhost')
    assert.equal(list.pathname, '/api/v1/my-actors')
    assert.equal(list.searchParams.get('scope'), 'collection')
    assert.equal(list.searchParams.get('sort'), 'progress')
    assert.equal(list.searchParams.get('query'), 'Шаламе')
    await actorsApi.profile(12)
    assert.equal(calls[1].url, '/api/v1/actors/12/profile')
    await actorsApi.search('Том')
    const search = new URL(calls[2].url, 'http://localhost')
    assert.equal(search.pathname, '/api/v1/people/search')
    assert.equal(search.searchParams.get('role'), 'actor')
    assert.equal(search.searchParams.get('query'), 'Том')
    await actorsApi.importPerson(1190668)
    assert.equal(calls[3].url, '/api/v1/actors/import')
    assert.equal(calls[3].init?.method, 'POST')
    assert.equal(calls[3].init?.body, '{"tmdb_id":1190668}')
    await actorsApi.followByTmdb(1190668)
    assert.equal(calls[4].url, '/api/v1/actors/follow-by-tmdb')
    await actorsApi.follow(12, true)
    await actorsApi.follow(12, false)
    assert.equal(calls[5].url, '/api/v1/actors/12/follow')
    assert.equal(calls[5].init?.method, 'PUT')
    assert.equal(calls[6].init?.method, 'DELETE')
    // Поиск режиссёров остаётся без role — как до появления актёров.
    await directorsApi.search('')
    assert.equal(calls[7].url, '/api/v1/people/search?')
    await directorsApi.myPeople({ scope: 'all', sort: 'name' })
    assert.equal(
      new URL(calls[8].url, 'http://localhost').pathname,
      '/api/v1/my-directors',
    )
  } finally {
    if (descriptor) Object.defineProperty(globalThis, 'window', descriptor)
    else Reflect.deleteProperty(globalThis, 'window')
  }
})
