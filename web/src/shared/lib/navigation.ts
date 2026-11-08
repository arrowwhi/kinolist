export type Navigate = (path: string, replace?: boolean) => void

// Карточка тайтла из каталога, которого ещё нет в коллекции:
// /catalog/{provider}/{media_type}/{external_id}?director={id} (или ?actor={id}).
// director/actor — профиль, из фильмографии которого открыли карточку.
export interface CatalogRoute {
  provider: 'tmdb' | 'kinopoisk'
  mediaType: 'movie' | 'tv'
  externalId: number
  directorId?: number
  actorId?: number
}

export function catalogPath(route: CatalogRoute): string {
  const path = `/catalog/${route.provider}/${route.mediaType}/${route.externalId}`
  if (route.directorId) return `${path}?director=${route.directorId}`
  if (route.actorId) return `${path}?actor=${route.actorId}`
  return path
}

function positiveId(value: string | null) {
  const id = Number(value)
  return Number.isInteger(id) && id > 0 ? id : undefined
}

export function parseCatalogRoute(location: string): CatalogRoute | null {
  const [path, query] = location.split('?')
  const match = path.match(/^\/catalog\/(tmdb|kinopoisk)\/(movie|tv)\/(\d+)$/)
  if (!match || !(Number(match[3]) > 0)) return null
  const params = new URLSearchParams(query)
  const director = positiveId(params.get('director'))
  const actor = positiveId(params.get('actor'))
  return {
    provider: match[1] as CatalogRoute['provider'],
    mediaType: match[2] as CatalogRoute['mediaType'],
    externalId: Number(match[3]),
    ...(director ? { directorId: director } : actor ? { actorId: actor } : {}),
  }
}

// Разделы людей: список, поиск в каталоге и профиль.
export type PersonKind = 'director' | 'actor'
export const peopleBase: Record<PersonKind, string> = {
  director: '/v2/directors',
  actor: '/actors',
}

export function personPath(kind: PersonKind, id: number) {
  return `${peopleBase[kind]}/${id}`
}

export function addPersonPath(kind: PersonKind, query = '') {
  const path = `${peopleBase[kind]}/add`
  return query ? `${path}?q=${encodeURIComponent(query)}` : path
}

export type PeopleRoute =
  | { kind: PersonKind; page: 'list' }
  | { kind: PersonKind; page: 'add' }
  | { kind: PersonKind; page: 'profile'; id: number }

// Путь без query-строки → страница раздела людей или null.
export function parsePeopleRoute(path: string): PeopleRoute | null {
  for (const kind of Object.keys(peopleBase) as PersonKind[]) {
    const base = peopleBase[kind]
    if (path === base) return { kind, page: 'list' }
    if (path === `${base}/add`) return { kind, page: 'add' }
    if (!path.startsWith(`${base}/`)) continue
    const rest = path.slice(base.length + 1)
    const id = /^\d+$/.test(rest) ? positiveId(rest) : undefined
    if (id) return { kind, page: 'profile', id }
  }
  return null
}
