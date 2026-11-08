import type { ActorFilmographyItem } from '../../api/actors'
import type { FilmographyItem } from '../../api/people'
import type { CastMember } from '../../api/types'
import { plural } from '../../shared/lib/format.ts'
import { mediaLabel } from '../person/metadata.ts'

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

export function castKey(member: CastMember, index: number) {
  return member.id
    ? `id:${member.id}`
    : member.tmdb_id
      ? `tmdb:${member.tmdb_id}`
      : `i:${index}`
}

// Как открыть профиль участника состава: по нашему id сразу, по tmdb_id —
// после import, без обоих — никак (только Кинопоиск).
export function castTarget(
  member: CastMember,
): { id: number } | { tmdbId: number } | null {
  if (member.id) return { id: member.id }
  if (member.tmdb_id) return { tmdbId: member.tmdb_id }
  return null
}
