import { useState } from 'react'
import { actorsApi } from '../../api/actors'
import type { CastMember } from '../../api/types'
import { castTarget } from '../../entities/actor/metadata'
import { useMutation } from '../../shared/hooks/useMutation'
import { personPath } from '../../shared/lib/navigation'
import type { Navigate } from '../../shared/lib/navigation'

// Тап по актёру в блоке «В ролях»: заведённый актёр открывается сразу,
// остальных сначала импортируем по tmdb_id (POST /actors/import).
export function useOpenActor(navigate: Navigate) {
  const mutation = useMutation()
  const [pending, setPending] = useState<string | null>(null)
  const open = (member: CastMember, key: string) => {
    const target = castTarget(member)
    if (!target) return
    if ('id' in target) {
      navigate(personPath('actor', target.id))
      return
    }
    setPending(key)
    void mutation
      .execute(
        () => actorsApi.importPerson(target.tmdbId),
        (actor) => navigate(personPath('actor', actor.id)),
      )
      .finally(() => setPending(null))
  }
  return { open, pending, error: mutation.error }
}
