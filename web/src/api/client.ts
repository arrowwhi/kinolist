import type { Genre, Job, MovieList, Status, UserMovie } from './types'

export class RequestError extends Error {
  readonly code: string

  constructor(code: string, message: string) {
    super(message)
    this.code = code
  }
}

function initData(): string {
  return window.Telegram?.WebApp?.initData ?? ''
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      // Подпись Telegram — единственное, чем клиент себя удостоверяет.
      'X-Telegram-Init-Data': initData(),
      ...(init.headers ?? {}),
    },
  })

  if (response.status === 204) {
    return undefined as T
  }

  const body = await response.json().catch(() => null)

  if (!response.ok) {
    throw new RequestError(
      body?.code ?? 'unknown',
      body?.message ?? 'Что-то пошло не так',
    )
  }

  return body as T
}

export interface ListParams {
  status?: Status
  genreIds?: number[]
  query?: string
  sort?: 'added_at' | 'title' | 'year' | 'rating'
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
}

export const api = {
  listMovies(params: ListParams = {}): Promise<MovieList> {
    const search = new URLSearchParams()
    if (params.status) search.set('status', params.status)
    if (params.genreIds?.length) search.set('genre_ids', params.genreIds.join(','))
    if (params.query) search.set('q', params.query)
    if (params.sort) search.set('sort', params.sort)
    if (params.order) search.set('order', params.order)
    if (params.limit) search.set('limit', String(params.limit))
    if (params.offset) search.set('offset', String(params.offset))

    const qs = search.toString()
    return request<MovieList>(`/movies${qs ? `?${qs}` : ''}`)
  },

  genres(): Promise<{ items: Genre[] }> {
    return request<{ items: Genre[] }>('/genres')
  },

  updateMovie(
    id: number,
    patch: { status?: Status; user_rating?: number; note?: string },
  ): Promise<UserMovie> {
    return request<UserMovie>(`/movies/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    })
  },

  deleteMovie(id: number): Promise<void> {
    return request<void>(`/movies/${id}`, { method: 'DELETE' })
  },

  createJob(input: string): Promise<Job> {
    return request<Job>('/jobs', { method: 'POST', body: JSON.stringify({ input }) })
  },

  getJob(id: number): Promise<Job> {
    return request<Job>(`/jobs/${id}`)
  },

  confirmJob(id: number, provider: string, externalId: number): Promise<UserMovie> {
    return request<UserMovie>(`/jobs/${id}/confirm`, {
      method: 'POST',
      body: JSON.stringify({ provider, external_id: externalId }),
    })
  },

  rejectJob(id: number): Promise<void> {
    return request<void>(`/jobs/${id}/reject`, { method: 'POST' })
  },
}
