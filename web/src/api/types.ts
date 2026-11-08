export type MediaType = 'movie' | 'tv'
export type Status = 'want_to_watch' | 'watched'

export interface Genre {
  id: number
  name_ru: string
  name_en: string
}

export interface Director {
  id?: number
  tmdb_id?: number
  kinopoisk_id?: number
  name: string
  original_name: string
}
export interface DirectorStats extends Director {
  id: number
  total_count: number
  want_to_watch_count: number
  watched_count: number
  rated_count: number
  average_user_rating: number | null
}
export interface DirectorList {
  items: DirectorStats[]
  total: number
  limit: number
  offset: number
}
export type DirectorSelection = Pick<DirectorStats, 'id' | 'name'>

export interface Movie {
  directors?: Director[]
  media_type?: MediaType
  season_count?: number | null
  id: number
  tmdb_id?: number
  kinopoisk_id?: number
  title: string
  original_title?: string
  year?: number
  overview: string
  poster_url?: string
  runtime_min?: number
  rating_provider?: 'tmdb' | 'kinopoisk'
  rating_vote_count: number | null
  rating?: number
  genres: Genre[]
}

export interface UserMovie {
  id: number
  movie: Movie
  status: Status
  user_rating?: number
  review: string
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
  summary: {
    movie_count: number
    rated_count: number
    average_user_rating: number | null
  }
}

export interface Candidate {
  directors?: Director[]
  media_type?: MediaType
  season_count?: number | null
  original_title?: string
  runtime_min?: number
  genres: Genre[]
  provider: string
  external_id: number
  title: string
  year?: number
  overview: string
  poster_url?: string
  rating_provider?: 'tmdb' | 'kinopoisk'
  rating_vote_count: number | null
  rating?: number
}

export type JobStatus =
  'pending' | 'processing' | 'confirming' | 'done' | 'failed' | 'cancelled'

export interface Job {
  id: number
  kind: 'reel' | 'title'
  status: JobStatus
  query_title?: string
  query_year?: number
  source_url?: string
  candidates: Candidate[]
  error?: string
  /** Причина неудачи при status=failed: title_not_recognized, no_candidates, reel_unavailable… */
  error_code?: string
}

export interface GenreStats extends Genre {
  want_to_watch_count: number
  watched_count: number
}
export interface ConfirmOptions {
  media_type?: MediaType
  status?: Status
  user_rating?: number
  note?: string
  review?: string
}

export interface Me {
  id: number
  username?: string
  first_name?: string
}
export interface Statistics {
  total_count: number
  want_to_watch_count: number
  watched_count: number
}
