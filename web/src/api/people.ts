// Общая часть разделов «Мои режиссёры» и «Мои актёры»: подписки, профиль
// с полной фильмографией из каталога и поиск людей в TMDB. Разделы отличаются
// только префиксом пути и ролью в поиске.
import { request } from './client.ts'
import { catalogApi } from './catalog.ts'
import type { MediaType, Status } from './types'

export interface PersonOverview {
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

export interface PersonOverviewList {
  items: PersonOverview[]
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

export interface PersonProfile<
  W extends FilmographyItem = FilmographyItem,
> extends PersonOverview {
  biography: string
  biography_lang?: string
  // Каталог не ответил — показан последний сохранённый снимок.
  stale: boolean
  filmography: W[]
}

export interface PersonSearchResult {
  // null — человек ещё не заведён в справочнике раздела, сначала нужен import.
  id: number | null
  tmdb_id: number
  name: string
  original_name: string
  photo_url?: string
  known_for_department?: string
  known_for: string[]
  followed: boolean
}

export type MyPeopleScope = 'all' | 'followed' | 'collection'
export type MyPeopleSort = 'name' | 'progress' | 'user_rating'
export type PersonRole = 'director' | 'actor'

export function filmographyKey(item: FilmographyItem) {
  return item.tmdb_id
    ? `${item.media_type}:${item.tmdb_id}`
    : `movie:${item.movie_id}`
}

// Пути разделов: /my-{section}, /{section}/{id}/profile и т. д.
const sections: Record<PersonRole, string> = {
  director: 'directors',
  actor: 'actors',
}

export function createPeopleApi<W extends FilmographyItem>(role: PersonRole) {
  const section = sections[role]
  return {
    myPeople(
      params: { scope: MyPeopleScope; sort: MyPeopleSort; query?: string },
      signal?: AbortSignal,
    ) {
      const search = new URLSearchParams({
        scope: params.scope,
        sort: params.sort,
        limit: '200',
      })
      if (params.query) search.set('query', params.query)
      return request<PersonOverviewList>(`/my-${section}?${search}`, {
        signal,
        cache: 'no-store',
      })
    },
    profile(id: number, signal?: AbortSignal) {
      return request<PersonProfile<W>>(`/${section}/${id}/profile`, {
        signal,
        cache: 'no-store',
      })
    },
    // Пустой запрос — подсказки: люди этой роли из коллекции без подписки.
    // Режиссёры — роль по умолчанию, поэтому их запрос остаётся прежним.
    search(query: string, signal?: AbortSignal) {
      const search = new URLSearchParams()
      if (query) search.set('query', query)
      if (role !== 'director') search.set('role', role)
      return request<{ items: PersonSearchResult[] }>(
        `/people/search?${search}`,
        { signal, cache: 'no-store' },
      ).then((r) => r.items)
    },
    importPerson(tmdbId: number) {
      return request<PersonOverview>(`/${section}/import`, {
        method: 'POST',
        body: JSON.stringify({ tmdb_id: tmdbId }),
      })
    },
    followByTmdb(tmdbId: number) {
      return request<PersonOverview>(`/${section}/follow-by-tmdb`, {
        method: 'POST',
        body: JSON.stringify({ tmdb_id: tmdbId }),
      })
    },
    follow(id: number, value: boolean) {
      return request<void>(`/${section}/${id}/follow`, {
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
}

export type PeopleApi<W extends FilmographyItem = FilmographyItem> = ReturnType<
  typeof createPeopleApi<W>
>
