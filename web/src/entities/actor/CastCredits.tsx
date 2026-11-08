import { useState } from 'react'
import type { CastMember } from '../../api/types'
import { PersonAvatar } from '../person/PersonAvatar'
import { castKey, castPreviewSize, castTarget } from './metadata'

// Блок «В ролях» карточки (макет «33 · Карточка / в ролях»): первые четыре
// актёра, остальные раскрываются прямо в карточке. В старых ответах состава
// нет — тогда блок не показываем.
export function CastCredits({
  cast = [],
  pending,
  onOpen,
}: {
  cast?: CastMember[]
  // Ключ участника (castKey), чей профиль сейчас заводится.
  pending?: string | null
  onOpen?: (member: CastMember, key: string) => void
}) {
  const [expanded, setExpanded] = useState(false)
  if (!cast.length) return null
  const visible = expanded ? cast : cast.slice(0, castPreviewSize)
  return (
    <section className="cast-credits" aria-labelledby="cast-title">
      <h2 id="cast-title" className="section-label">
        В ролях
      </h2>
      <ul className="cast-credits__grid">
        {visible.map((member, index) => {
          const key = castKey(member, index)
          const body = (
            <>
              <PersonAvatar
                name={member.name}
                url={member.photo_url}
                size="cast"
              />
              <span className="cast-member__name">{member.name}</span>
              {member.character && (
                <span className="cast-member__role">{member.character}</span>
              )}
            </>
          )
          const busy = pending === key
          return (
            <li key={key}>
              {onOpen && castTarget(member) ? (
                <button
                  type="button"
                  className="cast-member"
                  aria-busy={busy || undefined}
                  disabled={pending != null}
                  aria-label={
                    member.character
                      ? `${member.name} — ${member.character}`
                      : member.name
                  }
                  onClick={() => onOpen(member, key)}
                >
                  {body}
                  {busy && <span className="spinner" aria-hidden="true" />}
                </button>
              ) : (
                <div className="cast-member">{body}</div>
              )}
            </li>
          )
        })}
      </ul>
      {cast.length > castPreviewSize && (
        <button
          type="button"
          className="cast-credits__toggle"
          aria-expanded={expanded}
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? 'Свернуть' : 'Весь актёрский состав →'}
        </button>
      )}
    </section>
  )
}
