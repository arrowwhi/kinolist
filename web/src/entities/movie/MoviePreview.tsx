import { DirectorCredits } from '../director/DirectorCredits'
import { CastCredits } from '../actor/CastCredits'
import type { CastMember, Movie } from '../../api/types'
import { formatRuntime } from '../../shared/lib/format'
import { movieMetadata } from './metadata'
import { Poster } from './Poster'
import { Rating } from './Rating'
export function MoviePreview({
  movie,
  upcoming,
  onDirector,
  onActor,
  actorPending,
}: {
  movie: Movie
  // Тайтл ещё не вышел: вместо рейтинга — метка «Скоро · декабрь 2026».
  upcoming?: string
  onDirector?: (id: number) => void
  // Без onActor участники состава не кликабельны (экран подтверждения).
  onActor?: (member: CastMember, key: string) => void
  actorPending?: string | null
}) {
  return (
    <>
      <Poster
        url={movie.poster_url}
        title={movie.original_title || movie.title}
        size="wide"
      />
      <h2 className="preview-title">{movie.title}</h2>
      <p className="muted small">
        {[
          movieMetadata(movie),
          movie.media_type === 'tv' ? '' : formatRuntime(movie.runtime_min),
        ]
          .filter(Boolean)
          .join(' · ')}
      </p>
      <div className="preview-metadata">
        {upcoming ? (
          <span className="metadata-tag">{upcoming}</span>
        ) : (
          <Rating movie={movie} />
        )}
        {movie.genres.map((g) => (
          <span className="metadata-tag" key={g.id}>
            {g.name_ru}
          </span>
        ))}
      </div>
      <DirectorCredits directors={movie.directors} onOpen={onDirector} />
      <CastCredits cast={movie.cast} pending={actorPending} onOpen={onActor} />
      <p className="muted overview">
        {movie.overview || 'В каталоге пока нет описания.'}
      </p>
    </>
  )
}
