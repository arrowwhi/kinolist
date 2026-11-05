import type {
  Candidate,
  ConfirmOptions,
  Genre,
  GenreStats,
  Job,
  Movie,
  MovieList,
  Status,
  UserMovie,
} from './types'
export class RequestError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly existingUserMovieId?: number,
  ) {
    super(message)
  }
}
async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-Telegram-Init-Data': window.Telegram?.WebApp?.initData ?? '',
      ...init.headers,
    },
  })
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
  status?: Status
  genreIds?: number[]
  query?: string
  sort?: 'added_at' | 'title' | 'year' | 'rating' | 'user_rating' | 'watched_at'
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}
export const api = {
  listMovies(
    params: ListParams = {},
    signal?: AbortSignal,
  ): Promise<MovieList> {
    const search = new URLSearchParams()
    if (params.status) search.set('status', params.status)
    if (params.genreIds?.length)
      search.set('genre_ids', params.genreIds.join(','))
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
  collectionGenres(signal?: AbortSignal) {
    return request<{ items: GenreStats[] }>('/genres?scope=collection', {
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
    candidate: Pick<Candidate, 'provider' | 'external_id'>,
    signal?: AbortSignal,
  ) {
    return request<Movie>(
      `/jobs/${id}/candidates/${encodeURIComponent(candidate.provider)}/${candidate.external_id}`,
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
