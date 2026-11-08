import { safeExternalUrl, shortUrl } from '../../shared/lib/format'
import { openExternalLink } from '../../shared/lib/telegram'
import { ReelIcon } from './ReelIcon'

// Откуда фильм попал в коллекцию. Ссылка хранится отдельным полем
// (user_movies.source_url), а не в заметке: заметку пользователь
// переписывает, а источник должен пережить любые правки.
// Макет — Figma, экран 26 «Карточка / источник — рилс».
export function MovieSource({ source, url }: { source: string; url?: string }) {
  const href = safeExternalUrl(url)
  if (!href) return null
  return (
    <a
      className="movie-source"
      href={href}
      target="_blank"
      rel="noreferrer"
      onClick={(e) => {
        if (openExternalLink(href)) e.preventDefault()
      }}
    >
      <ReelIcon />
      <span className="movie-source__text">
        <span className="movie-source__title">
          {source === 'reel' ? 'Рилс в Instagram' : 'Ссылка'}
        </span>
        <span className="movie-source__url">{shortUrl(href)}</span>
      </span>
      <span className="movie-source__open" aria-hidden="true">
        ↗
      </span>
    </a>
  )
}
