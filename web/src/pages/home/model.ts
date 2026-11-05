export function greeting(firstName?: string) {
  const name = firstName?.trim()
  return name ? `Привет, ${name}!` : 'Привет!'
}
