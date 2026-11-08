// Раздел «Мои режиссёры»: подписки и полная фильмография из каталога.
import { createPeopleApi } from './people.ts'
import type {
  FilmographyItem,
  PersonOverview,
  PersonOverviewList,
  PersonProfile,
} from './people'

export type DirectorOverview = PersonOverview
export type DirectorOverviewList = PersonOverviewList
export type DirectorProfile = PersonProfile<FilmographyItem>

export const directorsApi = createPeopleApi<FilmographyItem>('director')
