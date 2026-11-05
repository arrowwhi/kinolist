export function errorMessage(
  error: unknown,
  fallback = 'Не удалось загрузить данные',
) {
  return error instanceof Error ? error.message : fallback
}
