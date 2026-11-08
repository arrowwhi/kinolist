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
// Русское склонение по числу: forms — для 1, 2 и 5 («фильм», «фильма»,
// «фильмов»). Единственное место с этим правилом.
export function plural(
  count: number,
  forms: readonly [string, string, string],
) {
  const last = Math.abs(count) % 100
  return last >= 11 && last <= 14
    ? forms[2]
    : last % 10 === 1
      ? forms[0]
      : last % 10 >= 2 && last % 10 <= 4
        ? forms[1]
        : forms[2]
}
/** Ссылка без схемы, www и хвостового слэша — для показа, не для перехода. */
export function shortUrl(url: string): string {
  return url
    .replace(/^https?:\/\//i, '')
    .replace(/^www\./i, '')
    .replace(/\/$/, '')
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
