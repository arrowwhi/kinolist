import { useState } from 'react'
import type { Director } from '../../api/types'
import { personKey, personTarget } from '../person/metadata'
export function DirectorCredits({
  directors = [],
  pending,
  onOpen,
}: {
  directors?: Director[]
  // Ключ режиссёра (personKey), чей профиль сейчас заводится.
  pending?: string | null
  onOpen?: (director: Director, key: string) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const visible = expanded ? directors : directors.slice(0, 3)
  return (
    <section
      className="director-credits stack stack--tight"
      aria-label="Режиссёры"
    >
      <h2>{directors.length > 1 ? 'Режиссёры' : 'Режиссёр'}</h2>
      {!directors.length ? (
        <p className="muted">Режиссёр не указан</p>
      ) : (
        <div className="director-credits__names">
          {visible.map((d, index) =>
            onOpen && personTarget(d) ? (
              <button
                type="button"
                key={personKey(d, index)}
                aria-busy={pending === personKey(d, index) || undefined}
                disabled={pending != null}
                onClick={() => onOpen(d, personKey(d, index))}
              >
                {d.name}
                <span aria-hidden="true"> →</span>
                {pending === personKey(d, index) && (
                  <span className="spinner" aria-hidden="true" />
                )}
              </button>
            ) : (
              <span key={`${d.tmdb_id ?? d.kinopoisk_id ?? index}:${index}`}>
                {d.name}
              </span>
            ),
          )}
        </div>
      )}
      {directors.length > 3 && (
        <button
          type="button"
          className="director-credits__toggle"
          aria-expanded={expanded}
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? 'Свернуть' : `Все режиссёры · ${directors.length}`}
        </button>
      )}
    </section>
  )
}
