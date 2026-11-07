import { api } from '../../api/client'
import type { DirectorListParams } from '../../api/client'
import { usePaginated } from '../../shared/hooks/usePaginated'
export function useDirectors(params: DirectorListParams, revision: number) {
  return usePaginated(
    (offset, signal) => api.listDirectors({ ...params, offset }, signal),
    JSON.stringify([params, revision]),
  )
}
