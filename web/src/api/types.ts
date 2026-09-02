export type Status = 'want_to_watch' | 'watched'

export interface Genre {
  id: number
  name_ru: string
  name_en: string
}

export interface Movie {
  id: number
  tmdb_id?: number
  kinopoisk_id?: number
  title: string
  original_title?: string
  year?: number
  overview: string
  poster_url?: string
  runtime_min?: number
  rating?: number
  genres: Genre[]
}

export interface UserMovie {
  id: number
  movie: Movie
  status: Status
  user_rating?: number
  note?: string
  source: string
  source_url?: string
  added_at: string
  watched_at?: string
}

export interface MovieList {
  items: UserMovie[]
  total: number
  limit: number
  offset: number
}

export interface Candidate {
  provider: string
  external_id: number
  title: string
  year?: number
  overview: string
  poster_url?: string
  rating?: number
}

export type JobStatus =
  | 'pending'
  | 'processing'
  | 'confirming'
  | 'done'
  | 'failed'
  | 'cancelled'

export interface Job {
  id: number
  kind: 'reel' | 'title'
  status: JobStatus
  query_title?: string
  query_year?: number
  source_url?: string
  candidates: Candidate[]
  error?: string
}
