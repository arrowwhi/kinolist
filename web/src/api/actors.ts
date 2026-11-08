// Раздел «Мои актёры»: устроен как «Мои режиссёры», но фильмография — роли.
import { createPeopleApi } from './people.ts'
import type {
  FilmographyItem,
  PersonOverview,
  PersonOverviewList,
  PersonProfile,
} from './people'

export type ActorOverview = PersonOverview
export type ActorOverviewList = PersonOverviewList

// Строка «Роли» в профиле актёра: работа плюс персонаж.
export interface ActorFilmographyItem extends FilmographyItem {
  // Несколько персонажей — через « / »; пустая строка, если неизвестен.
  character: string
}

export type ActorProfile = PersonProfile<ActorFilmographyItem>

export const actorsApi = createPeopleApi<ActorFilmographyItem>('actor')
