import type { Navigate } from '../../shared/lib/navigation'
import { useOpenPerson } from '../people/useOpenPerson'

// Тап по актёру в блоке «В ролях».
export function useOpenActor(navigate: Navigate) {
  return useOpenPerson('actor', navigate)
}
