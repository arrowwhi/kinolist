// Минимальное описание того, что реально используется из Telegram WebApp API.
interface TelegramThemeParams {
  bg_color?: string
  secondary_bg_color?: string
  text_color?: string
  hint_color?: string
  link_color?: string
  button_color?: string
  button_text_color?: string
  section_bg_color?: string
  header_bg_color?: string
}

interface TelegramHapticFeedback {
  impactOccurred(style: 'light' | 'medium' | 'heavy'): void
  notificationOccurred(type: 'error' | 'success' | 'warning'): void
  selectionChanged(): void
}

interface TelegramWebApp {
  BackButton?: {
    show(): void
    hide(): void
    onClick(handler: () => void): void
    offClick(handler: () => void): void
  }
  initData: string
  version?: string
  isVersionAtLeast?(version: string): boolean
  colorScheme: 'light' | 'dark'
  themeParams: TelegramThemeParams
  isExpanded: boolean
  platform?: string
  requestFullscreen?(): void
  disableVerticalSwipes?(): void
  ready(): void
  expand(): void
  close(): void
  HapticFeedback: TelegramHapticFeedback
  onEvent(event: string, handler: () => void): void
  setHeaderColor?(color: string): void
  setBackgroundColor?(color: string): void
  showConfirm?(message: string, callback: (ok: boolean) => void): void
}

interface Window {
  Telegram?: { WebApp?: TelegramWebApp }
}
