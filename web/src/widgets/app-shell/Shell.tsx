import type { ReactNode } from 'react'
import type { Navigate } from '../../shared/lib/navigation'
import { webApp, haptic } from '../../shared/lib/telegram'
export function Shell({
  title,
  subtitle,
  children,
  onBack,
  actions,
  tab,
  navigate,
}: {
  title: string
  subtitle?: string
  children: ReactNode
  onBack?: () => void
  actions?: ReactNode
  tab?: 'want_to_watch' | 'watched' | 'genres'
  navigate: Navigate
}) {
  return (
    <div className={`app-shell ${tab ? 'app-shell--tabs' : ''}`}>
      <header className="telegram-header">
        <button
          type="button"
          onClick={onBack ?? (() => webApp()?.close())}
          disabled={!onBack && !webApp()}
        >
          {onBack ? '← Назад' : 'Закрыть'}
        </button>
        <span>Кинолист</span>
        <span aria-hidden="true" />
      </header>
      <main className="page-content">
        <div className="page-heading">
          <h1>{title}</h1>
          {subtitle && <p className="muted">{subtitle}</p>}
        </div>
        {children}
      </main>
      {tab && (
        <footer className="bottom-bar">
          {actions}
          <nav aria-label="Основная навигация" className="bottom-nav">
            {(
              [
                ['want_to_watch', 'На потом', '/collection/want_to_watch'],
                ['watched', 'Просмотрено', '/collection/watched'],
                ['genres', 'Жанры', '/genres'],
              ] as const
            ).map(([key, name, path]) => (
              <a
                key={key}
                href={`#${path}`}
                aria-current={tab === key ? 'page' : undefined}
                onClick={(e) => {
                  e.preventDefault()
                  haptic('select')
                  navigate(path)
                }}
              >
                {name}
              </a>
            ))}
          </nav>
        </footer>
      )}
    </div>
  )
}
