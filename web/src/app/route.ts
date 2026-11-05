export const home = '/'

// Telegram launch parameters occupy the hash too. They are not app routes;
// leave the URL intact so the Telegram SDK can still read its launch data.
export function routeFromHash(hash: string): string {
  const value = hash.replace(/^#/, '')
  if (!value || value === '/') return home
  if (!value.startsWith('/')) {
    const params = new URLSearchParams(value)
    if ([...params.keys()].some((key) => key.startsWith('tgWebApp')))
      return home
  }
  return value
}
