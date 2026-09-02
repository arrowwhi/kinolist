import { useEffect } from 'react'

export function webApp() {
  return window.Telegram?.WebApp
}

export function haptic(type: 'light' | 'medium' | 'success' | 'error' | 'select') {
  const hf = webApp()?.HapticFeedback
  if (!hf) return

  switch (type) {
    case 'success':
      hf.notificationOccurred('success')
      break
    case 'error':
      hf.notificationOccurred('error')
      break
    case 'select':
      hf.selectionChanged()
      break
    default:
      hf.impactOccurred(type)
  }
}

// useTelegramSetup разворачивает мини-апп на весь экран и синхронизирует
// цвет шапки с темой клиента, иначе она остаётся белой в тёмной теме.
export function useTelegramSetup() {
  useEffect(() => {
    const app = webApp()
    if (!app) return

    app.ready()
    app.expand()

    const applyTheme = () => {
      const bg = app.themeParams.bg_color
      if (bg) {
        app.setHeaderColor?.(bg)
        app.setBackgroundColor?.(bg)
      }
    }

    applyTheme()
    app.onEvent('themeChanged', applyTheme)
  }, [])
}
