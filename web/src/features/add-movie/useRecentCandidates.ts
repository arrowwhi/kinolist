import { useCallback, useState } from 'react'
import type { Candidate } from '../../api/types'
import { candidateKey } from '../../entities/movie/metadata'
export interface RecentCandidate {
  input: string
  candidate: Candidate
}
// Session-only history: no personal searches are persisted or reused as job IDs.
export function useRecentCandidates() {
  const [recent, setRecent] = useState<RecentCandidate[]>([])
  const remember = useCallback((input: string, candidates: Candidate[]) => {
    setRecent((previous) => {
      const keys = new Set(candidates.map(candidateKey))
      return [
        ...candidates.map((candidate) => ({ input, candidate })),
        ...previous.filter((entry) => !keys.has(candidateKey(entry.candidate))),
      ].slice(0, 6)
    })
  }, [])
  return { recent, remember }
}
