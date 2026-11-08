export type Navigate = (path: string, replace?: boolean) => void

// Карточка тайтла из каталога, которого ещё нет в коллекции:
// /catalog/{provider}/{media_type}/{external_id}?director={id}.
// director — профиль, из фильмографии которого открыли карточку.
export interface CatalogRoute {
  provider: 'tmdb' | 'kinopoisk'
  mediaType: 'movie' | 'tv'
  externalId: number
  directorId?: number
}

export function catalogPath(route: CatalogRoute): string {
  const path = `/catalog/${route.provider}/${route.mediaType}/${route.externalId}`
  return route.directorId ? `${path}?director=${route.directorId}` : path
}

export function parseCatalogRoute(location: string): CatalogRoute | null {
  const [path, query] = location.split('?')
  const match = path.match(/^\/catalog\/(tmdb|kinopoisk)\/(movie|tv)\/(\d+)$/)
  if (!match || !(Number(match[3]) > 0)) return null
  const director = Number(new URLSearchParams(query).get('director'))
  return {
    provider: match[1] as CatalogRoute['provider'],
    mediaType: match[2] as CatalogRoute['mediaType'],
    externalId: Number(match[3]),
    ...(Number.isInteger(director) && director > 0
      ? { directorId: director }
      : {}),
  }
}
