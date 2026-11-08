// Каталог (TMDB/Кинопоиск) вне коллекции: карточка тайтла и добавление
// его в коллекцию без задачи распознавания.
import { request } from './client'
import type { CatalogMovie, MediaType, Status, UserMovie } from './types'

export type CatalogProvider = 'tmdb' | 'kinopoisk'

export interface AddFromCatalog {
  provider: CatalogProvider
  external_id: number
  media_type: MediaType
  status?: Status
  user_rating?: number
  review?: string
}

export const catalogApi = {
  get(
    provider: CatalogProvider,
    mediaType: MediaType,
    externalId: number,
    signal?: AbortSignal,
  ) {
    return request<CatalogMovie>(
      `/catalog/${provider}/${mediaType}/${externalId}`,
      { signal, cache: 'no-store' },
    )
  },
  // 409 с existing_user_movie_id — тайтл уже в коллекции.
  add(input: AddFromCatalog) {
    return request<UserMovie>('/movies', {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
}
