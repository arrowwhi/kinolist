import { useState } from 'react'
import { haptic, paintTelegram } from '../../shared/lib/telegram'
import {
  applyTheme,
  otherTheme,
  parseTheme,
  storeTheme,
} from '../../shared/lib/theme'

// Кнопка показывает тему, на которую переключит: в тёмной — солнце,
// в светлой — луну.
export function ThemeToggle() {
  const [theme, setTheme] = useState(() =>
    parseTheme(document.documentElement.dataset.theme),
  )
  const next = otherTheme(theme)
  return (
    <button
      type="button"
      className="theme-toggle"
      aria-label={next === 'light' ? 'Светлая тема' : 'Тёмная тема'}
      title={next === 'light' ? 'Светлая тема' : 'Тёмная тема'}
      onClick={() => {
        haptic('select')
        applyTheme(next)
        storeTheme(next)
        paintTelegram(next)
        setTheme(next)
      }}
    >
      {next === 'light' ? (
        <svg viewBox="0 0 18 18" width="18" height="18" aria-hidden="true">
          <g
            fill="none"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinecap="round"
          >
            <circle cx="9" cy="9" r="3.4" />
            <path d="M9 1.5v1.6M9 14.9v1.6M1.5 9h1.6M14.9 9h1.6M3.7 3.7l1.1 1.1M13.2 13.2l1.1 1.1M3.7 14.3l1.1-1.1M13.2 4.8l1.1-1.1" />
          </g>
        </svg>
      ) : (
        <svg viewBox="0 0 18 18" width="18" height="18" aria-hidden="true">
          <path
            d="M15.2 10.9A6.6 6.6 0 0 1 7.1 2.8a6.6 6.6 0 1 0 8.1 8.1Z"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinejoin="round"
          />
        </svg>
      )}
    </button>
  )
}
