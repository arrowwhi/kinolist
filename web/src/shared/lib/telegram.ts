import { useEffect, useRef } from 'react'
import { parseTheme, themeBackground, type Theme } from './theme'
export function webApp() {
  return window.Telegram?.WebApp
}
// Скрипт SDK подключён всегда, поэтому сам объект WebApp есть и в обычном
// браузере. Внутри Telegram его отличают непустые initData.
export function insideTelegram() {
  return Boolean(webApp()?.initData)
}
// Старые клиенты Telegram бросают исключение на методах новее своей версии,
// а не игнорируют их. Ни один вызов SDK не должен ронять приложение.
function call(minVersion: string, action: (app: TelegramWebApp) => void) {
  const app = webApp()
  if (!app) return
  try {
    if (app.isVersionAtLeast && !app.isVersionAtLeast(minVersion)) return
    action(app)
  } catch {
    // Косметика SDK не стоит падения интерфейса.
  }
}
export function haptic(type: 'select' | 'success' | 'error' | 'light') {
  call('6.1', (app) => {
    const feedback = app.HapticFeedback
    if (type === 'select') feedback.selectionChanged()
    else if (type === 'light') feedback.impactOccurred('light')
    else feedback.notificationOccurred(type)
  })
}
export function useTelegramSetup() {
  useEffect(() => {
    call('6.0', (app) => {
      app.ready()
      app.expand()
    })
    // На телефоне приложение занимает весь экран, как в макетах: сверху
    // остаются только статус-бар и плавающие кнопки Telegram. На десктопе
    // полноэкранный режим растянул бы окно на весь монитор — там не нужен.
    call('8.0', (app) => {
      if (app.platform === 'ios' || app.platform === 'android')
        app.requestFullscreen?.()
    })
    // Без шапки свайп вниз по списку закрывал бы приложение.
    call('7.7', (app) => app.disableVerticalSwipes?.())
    paintTelegram(parseTheme(document.documentElement.dataset.theme))
  }, [])
}
// Шапку и фон под страницей рисует клиент Telegram — красим их под тему,
// иначе при смене темы сверху останется полоса прежнего цвета.
export function paintTelegram(theme: Theme) {
  const color = themeBackground[theme]
  // Произвольный цвет шапки клиент понимает только с 6.9.
  call('6.9', (app) => app.setHeaderColor?.(color))
  call('6.1', (app) => app.setBackgroundColor?.(color))
}
// Нативная стрелка «назад» Telegram ведёт себя так же, как обработчик
// текущей страницы: шаг назад в мастере, защита от ухода во время запроса.
export function useTelegramBackButton(onBack: (() => void) | undefined) {
  const handler = useRef(onBack)
  handler.current = onBack
  const visible = Boolean(onBack)
  useEffect(() => {
    const click = () => handler.current?.()
    call('6.1', (app) => {
      if (!app.BackButton) return
      if (visible) {
        app.BackButton.show()
        app.BackButton.onClick(click)
      } else app.BackButton.hide()
    })
    return () =>
      call('6.1', (app) => {
        app.BackButton?.offClick(click)
        app.BackButton?.hide()
      })
  }, [visible])
}
