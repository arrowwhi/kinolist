import { useState } from 'react'
import { actorsApi } from '../../api/actors'
import { directorsApi } from '../../api/directors'
import { useMutation } from '../../shared/hooks/useMutation'
import { personPath } from '../../shared/lib/navigation'
import type { Navigate, PersonKind } from '../../shared/lib/navigation'
import { personTarget } from '../../entities/person/metadata'
import type { PersonRef } from '../../entities/person/metadata'

const apis = { actor: actorsApi, director: directorsApi }

// Тап по человеку в карточке: заведённый открывается сразу, остальных
// сначала импортируем по tmdb_id (POST /{actors|directors}/import).
export function useOpenPerson(kind: PersonKind, navigate: Navigate) {
  const mutation = useMutation()
  const [pending, setPending] = useState<string | null>(null)
  const open = (person: PersonRef, key: string) => {
    const target = personTarget(person)
    if (!target) return
    if ('id' in target) {
      navigate(personPath(kind, target.id))
      return
    }
    setPending(key)
    void mutation
      .execute(
        () => apis[kind].importPerson(target.tmdbId),
        (imported) => navigate(personPath(kind, imported.id)),
      )
      .finally(() => setPending(null))
  }
  return { open, pending, error: mutation.error }
}
