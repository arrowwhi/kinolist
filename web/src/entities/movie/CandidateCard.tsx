import type { Candidate } from '../../api/types'
import { Poster } from './Poster'
import { Rating } from './Rating'
import { movieMetadata } from './metadata'
export function CandidateCard({
  candidate,
  onSelect,
  disabled = false,
}: {
  candidate: Candidate
  onSelect: () => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      className="movie-card"
      disabled={disabled}
      onClick={onSelect}
    >
      <Poster title={candidate.title} url={candidate.poster_url} />
      <span className="movie-card__body">
        <strong className="movie-card__title">{candidate.title}</strong>
        <span className="movie-card__meta">
          {movieMetadata(candidate, { search: true })}
        </span>
        <span className="movie-card__rating">
          <Rating movie={candidate} />
        </span>
        <span className="movie-card__date">Открыть подробности →</span>
      </span>
    </button>
  )
}
