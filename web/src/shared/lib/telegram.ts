import { useEffect } from 'react'
export function webApp() {
  return window.Telegram?.WebApp
}
export function haptic(type: 'select' | 'success' | 'error' | 'light') {
  const feedback = webApp()?.HapticFeedback
  if (!feedback) return
  if (type === 'select') feedback.selectionChanged()
  else if (type === 'light') feedback.impactOccurred('light')
  else feedback.notificationOccurred(type)
}
export function useTelegramSetup(onBack: () => void, canGoBack: boolean) {
  useEffect(() => {
    const app = webApp()
    app?.ready()
    app?.expand()
    app?.setHeaderColor?.('#111311')
    app?.setBackgroundColor?.('#111311')
  }, [])
  useEffect(() => {
    const button = webApp()?.BackButton
    if (!button || !canGoBack) {
      button?.hide()
      return
    }
    button.show()
    button.onClick(onBack)
    return () => {
      button.offClick(onBack)
      button.hide()
    }
  }, [onBack, canGoBack])
}
