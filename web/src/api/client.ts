import type {
  Candidate,
  DirectorStats,
  DirectorList,
  ConfirmOptions,
  Genre,
  GenreStats,
  Job,
  Movie,
  Me,
  Statistics,
  MovieList,
  MediaType,
  Status,
  UserMovie,
} from './types'
export class RequestError extends Error {
  readonly code: string
  readonly existingUserMovieId?: number
  constructor(code: string, message: string, existingUserMovieId?: number) {
    super(message)
    this.code = code
    this.existingUserMovieId = existingUserMovieId
  }
}
async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let response: Response
  try {
    response = await fetch(`/api/v1${path}`, {
      ...init,
      headers: {
        'Content-Type': 'application/json',
        'X-Telegram-Init-Data': window.Telegram?.WebApp?.initData ?? '',
        ...init.headers,
      },
    })
  } catch (err) {
    // Отмену запроса вызывающий распознаёт сам, а сетевой сбой браузер
    // описывает по-английски («Failed to fetch») — подменяем понятным текстом.
    if (init.signal?.aborted) throw err
    throw new RequestError(
      'network',
      'Нет связи с сервером. Проверь интернет и попробуй ещё раз.',
    )
  }
  if (response.status === 204) return undefined as T
  const body = await response.json().catch(() => null)
  if (!response.ok)
    throw new RequestError(
      body?.code ?? 'unknown',
      body?.message ?? 'Не удалось выполнить запрос. Попробуйте ещё раз.',
      body?.existing_user_movie_id,
    )
  return body as T
}
export interface ListParams {
  mediaType?: MediaType
  status?: Status
  genreIds?: number[]
  directorIds?: number[]
  query?: string
  sort?: 'added_at' | 'title' | 'year' | 'rating' | 'user_rating' | 'watched_at'
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}
export interface DirectorListParams {
  query?: string
  mediaType?: MediaType
  sort?: 'name' | 'work_count' | 'average_user_rating'
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}
export const api = {
  listDirectors(params: DirectorListParams = {}, signal?: AbortSignal) {
    const query = new URLSearchParams()
    if (params.query) query.set('q', params.query)
    if (params.mediaType) query.set('media_type', params.mediaType)
    if (params.sort) query.set('sort', params.sort)
    if (params.order) query.set('order', params.order)
    if (params.limit) query.set('limit', String(params.limit))
    if (params.offset) query.set('offset', String(params.offset))
    return request<DirectorList>(`/directors?${query}`, {
      signal,
      cache: 'no-store',
    })
  },
  getDirector(id: number, signal?: AbortSignal) {
    return request<DirectorStats>(`/directors/${id}`, {
      signal,
      cache: 'no-store',
    })
  },
  me(signal?: AbortSignal) {
    return request<Me>('/me', { signal })
  },
  statistics(signal?: AbortSignal) {
    return request<Statistics>('/statistics', { signal, cache: 'no-store' })
  },
  listMovies(
    params: ListParams = {},
    signal?: AbortSignal,
  ): Promise<MovieList> {
    const search = new URLSearchParams()
    if (params.status) search.set('status', params.status)
    if (params.mediaType) search.set('media_type', params.mediaType)
    if (params.genreIds?.length)
      search.set('genre_ids', params.genreIds.join(','))
    if (params.directorIds?.length)
      search.set('director_ids', params.directorIds.join(','))
    if (params.query) search.set('q', params.query)
    if (params.sort) search.set('sort', params.sort)
    if (params.order) search.set('order', params.order)
    if (params.limit) search.set('limit', String(params.limit))
    if (params.offset) search.set('offset', String(params.offset))
    return request<MovieList>(`/movies?${search}`, { signal })
  },
  genres(signal?: AbortSignal) {
    return request<{ items: Genre[] }>('/genres', { signal })
  },
  collectionGenres(
    signal?: AbortSignal,
    mediaType?: MediaType,
    directorIds?: number[],
  ) {
    const query = new URLSearchParams({ scope: 'collection' })
    if (directorIds?.length) query.set('director_ids', directorIds.join(','))
    if (mediaType) query.set('media_type', mediaType)
    return request<{ items: GenreStats[] }>(`/genres?${query}`, {
      signal,
    })
  },
  getMovie(id: number, signal?: AbortSignal) {
    return request<UserMovie>(`/movies/${id}`, { signal })
  },
  updateMovie(
    id: number,
    patch: {
      status?: Status
      user_rating?: number
      note?: string
      review?: string
    },
  ) {
    return request<UserMovie>(`/movies/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    })
  },
  deleteMovie(id: number) {
    return request<void>(`/movies/${id}`, { method: 'DELETE' })
  },
  createJob(input: string) {
    return request<Job>('/jobs', {
      method: 'POST',
      body: JSON.stringify({ input }),
    })
  },
  getJob(id: number, signal?: AbortSignal) {
    return request<Job>(`/jobs/${id}`, { signal })
  },
  candidateDetails(
    id: number,
    candidate: Pick<Candidate, 'provider' | 'external_id' | 'media_type'>,
    signal?: AbortSignal,
  ) {
    return request<Movie>(
      `/jobs/${id}/candidates/${encodeURIComponent(candidate.provider)}/${candidate.media_type ?? 'movie'}/${candidate.external_id}`,
      { signal },
    )
  },
  confirmJob(
    id: number,
    provider: string,
    externalId: number,
    options: ConfirmOptions = {},
  ) {
    return request<UserMovie>(`/jobs/${id}/confirm`, {
      method: 'POST',
      body: JSON.stringify({ provider, external_id: externalId, ...options }),
    })
  },
  rejectJob(id: number) {
    return request<void>(`/jobs/${id}/reject`, { method: 'POST' })
  },
}
