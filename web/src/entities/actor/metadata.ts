import type { ActorFilmographyItem } from '../../api/actors'
import type { FilmographyItem } from '../../api/people'
import { plural } from '../../shared/lib/format.ts'
import { mediaLabel, personKey, personTarget } from '../person/metadata.ts'

export function actorCount(count: number) {
  return `${count} ${plural(count, ['актёр', 'актёра', 'актёров'])}`
}

export function roleCount(count: number) {
  return `${count} ${plural(count, ['роль', 'роли', 'ролей'])}`
}

// Мета строки «Роли»: «2024 · Фильм · роль: Пол Атрейдес». Оригинальное
// название не показываем — место занимает персонаж.
export function roleMeta(
  item: Pick<FilmographyItem, 'year' | 'media_type' | 'episode_count'> &
    Partial<Pick<ActorFilmographyItem, 'character'>>,
) {
  const character = item.character?.trim()
  return [
    item.year ?? 'Год неизвестен',
    mediaLabel(item),
    character ? `роль: ${character}` : '',
  ]
    .filter(Boolean)
    .join(' · ')
}

// В карточке сначала видно четыре актёра, остальные — по ссылке.
export const castPreviewSize = 4

// Участник «В ролях» открывается так же, как любой человек карточки.
export { personKey as castKey, personTarget as castTarget }
