import type { ReactNode } from 'react'
import { ThemeToggle } from '../../features/theme/ThemeToggle'
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
  floating,
  tab,
  navigate,
  themeToggle,
}: {
  title: string
  subtitle?: ReactNode
  children: ReactNode
  onBack?: () => void
  // Действие, закреплённое у нижнего края на экранах без вкладок.
  floating?: ReactNode
  tab?: 'want_to_watch' | 'watched' | 'genres'
  navigate: Navigate
  // Переключатель темы — только на основных экранах, не на вложенных.
  themeToggle?: boolean
}) {
  useTelegramBackButton(onBack)
  // В Telegram шапку с «назад» и «закрыть» рисует сам клиент — своя
  // нужна только в обычном браузере, где нативной стрелки нет.
  const telegram = insideTelegram()
  return (
    <div
      className={[
        'app-shell',
        tab || floating ? 'app-shell--pinned' : '',
        telegram ? 'app-shell--telegram' : '',
      ].join(' ')}
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
          {themeToggle ? (
            <div className="page-heading__title">
              <h1>{title}</h1>
              <ThemeToggle />
            </div>
          ) : (
            <h1>{title}</h1>
          )}
          {subtitle && <p className="muted">{subtitle}</p>}
        </div>
        {children}
      </main>
      {floating && !tab && (
        <footer className="bottom-bar bottom-bar--center">{floating}</footer>
      )}
      {tab && (
        <footer className="bottom-bar">
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
          <button
            type="button"
            className="bottom-add"
            aria-label="Добавить фильм или сериал"
            onClick={() => {
              haptic('light')
              navigate('/add')
            }}
          >
            <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
              <path
                d="M12 4v16M4 12h16"
                stroke="currentColor"
                strokeWidth="2.5"
                strokeLinecap="round"
              />
            </svg>
          </button>
        </footer>
      )}
    </div>
  )
}
