import { RequestError } from '../../api/client'
// Показываем только тексты сервера: сообщения прочих ошибок (браузера,
// рантайма) не для пользователя и бывают на английском.
export function errorMessage(
  error: unknown,
  fallback = 'Не удалось загрузить данные',
) {
  return error instanceof RequestError ? error.message : fallback
}
