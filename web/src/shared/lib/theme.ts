// Тема хранится в localStorage этого устройства: на сервере её нет, а
// Telegram сохраняет localStorage Mini App между запусками.
export type Theme = 'dark' | 'light'

export const themeStorageKey = 'kinolist.theme'

// Цвет фона каждой темы — для шапки и фона клиента Telegram, которые
// не видят CSS-переменных. Совпадает с --color-bg в tokens.css.
export const themeBackground: Record<Theme, string> = {
  dark: '#111311',
  light: '#f4f5ef',
}

// Тёмная — утверждённый дизайн, поэтому всё, кроме явного «light»,
// включая пустое и испорченное значение, читается как тёмная.
export function parseTheme(value: string | null | undefined): Theme {
  return value === 'light' ? 'light' : 'dark'
}

export function otherTheme(theme: Theme): Theme {
  return theme === 'dark' ? 'light' : 'dark'
}

// В приватном режиме и при запрете хранилища доступ бросает исключение —
// тогда тема просто не запоминается.
export function readStoredTheme(): Theme {
  try {
    return parseTheme(window.localStorage.getItem(themeStorageKey))
  } catch {
    return 'dark'
  }
}

export function storeTheme(theme: Theme) {
  try {
    window.localStorage.setItem(themeStorageKey, theme)
  } catch {
    // Не запомнили — тема всё равно применится до перезапуска.
  }
}

export function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme
}
