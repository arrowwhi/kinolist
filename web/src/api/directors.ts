// Раздел «Мои режиссёры»: подписки и полная фильмография из каталога.
import { request } from './client'
import { catalogApi } from './catalog'
import type { MediaType, Status } from './types'

export interface DirectorOverview {
  id: number
  tmdb_id?: number
  kinopoisk_id?: number
  name: string
  original_name: string
  photo_url?: string
  birth_date?: string
  death_date?: string
  place_of_birth?: string
  known_for: string[]
  followed: boolean
  // false — фильмография из каталога ещё загружается, work_count пока
  // считает только работы из коллекции.
  filmography_complete: boolean
  work_count: number
  in_collection_count: number
  watched_count: number
  want_to_watch_count: number
  rated_count: number
  average_user_rating: number | null
}

export interface DirectorOverviewList {
  items: DirectorOverview[]
  total: number
  limit: number
  offset: number
  counts: { all: number; followed: number; collection: number }
}

export interface FilmographyItem {
  // Нет у работ, известных только Кинопоиску.
  tmdb_id?: number
  movie_id?: number
  media_type: MediaType
  title: string
  original_title: string
  year?: number
  release_date?: string
  poster_url?: string
  rating: number | null
  vote_count: number
  episode_count?: number
  upcoming: boolean
  user_movie_id?: number
  status?: Status
  user_rating?: number
}

export interface DirectorProfile extends DirectorOverview {
  biography: string
  biography_lang?: string
  // Каталог не ответил — показан последний сохранённый снимок.
  stale: boolean
  filmography: FilmographyItem[]
}

export interface PersonSearchResult {
  // null — человек ещё не заведён как режиссёр, сначала нужен import.
  id: number | null
  tmdb_id: number
  name: string
  original_name: string
  photo_url?: string
  known_for_department?: string
  known_for: string[]
  followed: boolean
}

export type MyDirectorsScope = 'all' | 'followed' | 'collection'
export type MyDirectorsSort = 'name' | 'progress' | 'user_rating'

export function filmographyKey(item: FilmographyItem) {
  return item.tmdb_id
    ? `${item.media_type}:${item.tmdb_id}`
    : `movie:${item.movie_id}`
}

export const directorsApi = {
  myDirectors(
    params: {
      scope: MyDirectorsScope
      sort: MyDirectorsSort
      query?: string
    },
    signal?: AbortSignal,
  ) {
    const search = new URLSearchParams({
      scope: params.scope,
      sort: params.sort,
      limit: '200',
    })
    if (params.query) search.set('query', params.query)
    return request<DirectorOverviewList>(`/my-directors?${search}`, {
      signal,
      cache: 'no-store',
    })
  },
  profile(id: number, signal?: AbortSignal) {
    return request<DirectorProfile>(`/directors/${id}/profile`, {
      signal,
      cache: 'no-store',
    })
  },
  // Пустой запрос — подсказки: режиссёры из коллекции без подписки.
  search(query: string, signal?: AbortSignal) {
    const search = new URLSearchParams()
    if (query) search.set('query', query)
    return request<{ items: PersonSearchResult[] }>(
      `/people/search?${search}`,
      { signal, cache: 'no-store' },
    ).then((r) => r.items)
  },
  importPerson(tmdbId: number) {
    return request<DirectorOverview>('/directors/import', {
      method: 'POST',
      body: JSON.stringify({ tmdb_id: tmdbId }),
    })
  },
  followByTmdb(tmdbId: number) {
    return request<DirectorOverview>('/directors/follow-by-tmdb', {
      method: 'POST',
      body: JSON.stringify({ tmdb_id: tmdbId }),
    })
  },
  follow(id: number, value: boolean) {
    return request<void>(`/directors/${id}/follow`, {
      method: value ? 'PUT' : 'DELETE',
    })
  },
  // Быстрое добавление работы из фильмографии в «На потом».
  // Кнопка «+» есть только у работ с tmdb_id.
  addToWatchlist(item: FilmographyItem) {
    return catalogApi.add({
      provider: 'tmdb',
      external_id: item.tmdb_id!,
      media_type: item.media_type,
    })
  },
}
