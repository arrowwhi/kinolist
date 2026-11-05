export function formatDate(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime())
    ? ''
    : new Intl.DateTimeFormat('ru-RU', {
        day: 'numeric',
        month: 'long',
        year: 'numeric',
      }).format(date)
}
export function formatRuntime(minutes?: number) {
  if (!minutes) return ''
  return [
    minutes >= 60 ? `${Math.floor(minutes / 60)} ч` : '',
    minutes % 60 ? `${minutes % 60} мин` : '',
  ]
    .filter(Boolean)
    .join(' ')
}
export function movieCount(count: number) {
  const mod = count % 100
  const word =
    mod >= 11 && mod <= 14
      ? 'фильмов'
      : count % 10 === 1
        ? 'фильм'
        : count % 10 >= 2 && count % 10 <= 4
          ? 'фильма'
          : 'фильмов'
  return `${count} ${word}`
}
export function safeExternalUrl(value?: string) {
  if (!value) return null
  try {
    const url = new URL(value)
    return ['https:', 'http:'].includes(url.protocol) ? url.href : null
  } catch {
    return null
  }
}
