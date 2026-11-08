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

const releaseMonths = [
  'январь',
  'февраль',
  'март',
  'апрель',
  'май',
  'июнь',
  'июль',
  'август',
  'сентябрь',
  'октябрь',
  'ноябрь',
  'декабрь',
] as const

// Метка ещё не вышедшего тайтла: «Скоро · декабрь 2026». release_date
// приходит как «2026-12-18», бывает и без дня или месяца; без даты — год.
export function upcomingLabel(releaseDate?: string, year?: number) {
  const match = releaseDate?.match(/^(\d{4})(?:-(\d{2}))?/)
  const releaseYear = match ? Number(match[1]) : year
  const month = match?.[2] ? releaseMonths[Number(match[2]) - 1] : undefined
  if (!releaseYear) return 'Скоро'
  return `Скоро · ${month ? `${month} ${releaseYear}` : releaseYear}`
}
