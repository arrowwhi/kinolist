import { useState } from 'react'
import type { Director } from '../../api/types'
export function DirectorCredits({
  directors = [],
  onOpen,
}: {
  directors?: Director[]
  onOpen?: (id: number) => void
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
            onOpen && d.id ? (
              <button type="button" key={d.id} onClick={() => onOpen(d.id!)}>
                {d.name}
                <span aria-hidden="true"> →</span>
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
