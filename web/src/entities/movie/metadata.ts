import type { Candidate, MediaType, Movie } from '../../api/types'
import { plural } from '../../shared/lib/format.ts'

type Metadata = Pick<Movie, 'media_type' | 'year' | 'season_count'>
export function mediaLabel(type?: MediaType) {
  return type === 'tv' ? 'Сериал' : 'Фильм'
}
export function mediaFilterLabel(type?: MediaType) {
  return type === 'tv' ? 'сериалы' : type === 'movie' ? 'фильмы' : 'все типы'
}
export function contentCount(count: number, type?: MediaType) {
  const forms =
    type === 'tv'
      ? (['сериал', 'сериала', 'сериалов'] as const)
      : type === 'movie'
        ? (['фильм', 'фильма', 'фильмов'] as const)
        : (['запись', 'записи', 'записей'] as const)
  return `${count} ${plural(count, forms)}`
}
export function seasonCount(count?: number | null) {
  return count != null && Number.isInteger(count) && count > 0
    ? `${count} ${plural(count, ['сезон', 'сезона', 'сезонов'])}`
    : 'Сезоны не указаны'
}
export function movieMetadata(movie: Metadata, { search = false } = {}) {
  const info = [mediaLabel(movie.media_type), movie.year]
  if (movie.media_type === 'tv') {
    if (!search || (movie.season_count != null && movie.season_count > 0))
      info.push(seasonCount(movie.season_count))
  }
  return info.filter(Boolean).join(' · ')
}
export function candidateKey(
  candidate: Pick<Candidate, 'provider' | 'media_type' | 'external_id'>,
) {
  return `${candidate.provider}:${candidate.media_type ?? 'movie'}:${candidate.external_id}`
}
