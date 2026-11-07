import type { ReactNode } from 'react'
import type { Navigate } from '../../shared/lib/navigation'
import {
  haptic,
  insideTelegram,
  useTelegramBackButton,
} from '../../shared/lib/telegram'
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
  subtitle?: ReactNode
  children: ReactNode
  onBack?: () => void
  actions?: ReactNode
  tab?: 'want_to_watch' | 'watched' | 'genres'
  navigate: Navigate
}) {
  useTelegramBackButton(onBack)
  // В Telegram шапку с «назад» и «закрыть» рисует сам клиент — своя
  // нужна только в обычном браузере, где нативной стрелки нет.
  const telegram = insideTelegram()
  return (
    <div
      className={`app-shell ${tab ? 'app-shell--tabs' : ''} ${telegram ? 'app-shell--telegram' : ''}`}
    >
      {!telegram && (
        <header className="telegram-header">
          {onBack ? (
            <button type="button" onClick={onBack}>
              ← Назад
            </button>
          ) : (
            <span aria-hidden="true" />
          )}
          <span>Кинолист</span>
          <span aria-hidden="true" />
        </header>
      )}
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
