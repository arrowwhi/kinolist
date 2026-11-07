import type { DirectorOverview } from '../../api/directors'
import { plural } from '../../shared/lib/format.ts'

type Lifespan = Pick<DirectorOverview, 'birth_date' | 'death_date'>

function year(date?: string) {
  return date ? Number(date.slice(0, 4)) || 0 : 0
}

export function lifespan(person: Lifespan) {
  const born = year(person.birth_date)
  const died = year(person.death_date)
  if (!born) return died ? `ум. ${died}` : ''
  return died ? `${born}–${died}` : `род. ${born}`
}

// Страна — последний фрагмент места рождения из TMDB («…, Квебек, Канада»).
export function country(placeOfBirth?: string) {
  return placeOfBirth?.split(',').at(-1)?.trim() ?? ''
}

export function personLine(
  person: Lifespan &
    Pick<DirectorOverview, 'name' | 'original_name' | 'place_of_birth'>,
) {
  return [
    person.original_name !== person.name ? person.original_name : '',
    lifespan(person),
    country(person.place_of_birth),
  ]
    .filter(Boolean)
    .join(' · ')
}

export function worksWatched(watched: number, total: number) {
  return `Посмотрено ${watched} из ${total}`
}

export function filmCount(count: number) {
  return `${count} ${plural(count, ['работа', 'работы', 'работ'])}`
}

export function progress(watched: number, total: number) {
  return total ? Math.round((watched / total) * 100) : 0
}
