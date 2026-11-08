import type { Navigate } from '../../shared/lib/navigation'
import { useOpenPerson } from '../people/useOpenPerson'

// Тап по режиссёру в карточке: у тайтла не из коллекции режиссёр часто ещё
// не заведён — тогда сначала import по tmdb_id.
export function useOpenDirector(navigate: Navigate) {
  return useOpenPerson('director', navigate)
}
