// Разделы людей. «Мои режиссёры» и «Мои актёры» — одни и те же страницы
// (pages/people), отличаются API, текстами и строкой фильмографии.
import { actorsApi } from '../../api/actors'
import { directorsApi } from '../../api/directors'
import type { FilmographyItem, PeopleApi } from '../../api/people'
import type { CatalogRoute, PersonKind } from '../../shared/lib/navigation'
import { directorCount } from '../../entities/director/metadata'
import { actorCount, roleCount, roleMeta } from '../../entities/actor/metadata'
import { filmCount, workMeta } from '../../entities/person/metadata'

export interface PeopleSection {
  kind: PersonKind
  api: PeopleApi<FilmographyItem>
  // Строка под названием работы в фильмографии.
  workMeta: (item: FilmographyItem) => string
  // Откуда открыта карточка каталога — для подписи «из фильмографии: …».
  catalogSource: (id: number) => Pick<CatalogRoute, 'directorId' | 'actorId'>
  list: {
    title: string
    subtitle: string
    add: string
    scopeLabel: string
    searchPlaceholder: string
    sortLabel: string
    collectionHint: string
    notFound: string
    emptyFollowed: string
    empty: string
    emptyDescription: string
    addAction: string
    count: (count: number) => string
  }
  profile: {
    fallbackTitle: string
    notFound: string
    heroLabel: string
    stale: string
    loading: string
    worksTitle: string
    worksCount: (count: number) => string
    allInCollection: string
  }
  search: {
    title: string
    subtitle: string
    placeholder: string
    label: string
    loading: string
    emptyTitle: string
    emptyDescription: string
    // Подсказка внизу экрана поиска (макет «32 · Добавить актёра»).
    hint?: [string, string]
  }
}

export const directorSection: PeopleSection = {
  kind: 'director',
  api: directorsApi,
  workMeta,
  catalogSource: (id) => ({ directorId: id }),
  list: {
    title: 'Режиссёры',
    subtitle: 'Следи за любимыми авторами и смотри их фильмографию целиком.',
    add: '＋ Добавить режиссёра',
    scopeLabel: 'Какие режиссёры',
    searchPlaceholder: 'Поиск среди моих режиссёров',
    sortLabel: 'Сортировка режиссёров',
    collectionHint:
      'Режиссёры фильмов из твоей коллекции. Отметь «Отслеживать» в профиле, чтобы закрепить автора.',
    notFound: 'Режиссёр не найден',
    emptyFollowed: 'Пока никого не отслеживаешь',
    empty: 'Здесь будут твои режиссёры',
    emptyDescription:
      'Добавь режиссёра, чтобы видеть всю его фильмографию и что уже посмотрено.',
    addAction: 'Добавить режиссёра',
    count: directorCount,
  },
  profile: {
    fallbackTitle: 'Режиссёр',
    notFound: 'Режиссёр не найден',
    heroLabel: 'О режиссёре',
    stale: 'Каталог сейчас не отвечает — показываем сохранённую фильмографию.',
    loading:
      'Полная фильмография ещё загружается — пока видны только работы из коллекции.',
    worksTitle: 'Фильмография',
    worksCount: filmCount,
    allInCollection: 'Все работы режиссёра уже в твоей коллекции.',
  },
  search: {
    title: 'Добавить режиссёра',
    subtitle:
      'Найди автора по имени — покажем всю его фильмографию, даже то, чего ещё нет в коллекции.',
    placeholder: 'Например, Вильнёв или Nolan',
    label: 'Имя режиссёра',
    loading: 'Ищем режиссёров',
    emptyTitle: 'Найди режиссёра по имени',
    emptyDescription:
      'Здесь появятся режиссёры фильмов из коллекции, на которых ты ещё не подписан.',
  },
}

export const actorSection: PeopleSection = {
  kind: 'actor',
  api: actorsApi,
  workMeta: roleMeta,
  catalogSource: (id) => ({ actorId: id }),
  list: {
    title: 'Актёры',
    subtitle: 'Следи за любимыми актёрами и смотри все их роли.',
    add: '＋ Добавить актёра',
    scopeLabel: 'Какие актёры',
    searchPlaceholder: 'Поиск среди моих актёров',
    sortLabel: 'Сортировка актёров',
    collectionHint:
      'Актёры, сыгравшие минимум в двух работах из твоей коллекции. Отметь «Отслеживать» в профиле, чтобы закрепить актёра.',
    notFound: 'Актёр не найден',
    emptyFollowed: 'Пока никого не отслеживаешь',
    empty: 'Здесь будут твои актёры',
    emptyDescription:
      'Добавь актёра, чтобы видеть все его роли и что уже посмотрено.',
    addAction: 'Добавить актёра',
    count: actorCount,
  },
  profile: {
    fallbackTitle: 'Актёр',
    notFound: 'Актёр не найден',
    heroLabel: 'Об актёре',
    stale: 'Каталог сейчас не отвечает — показываем сохранённые роли.',
    loading:
      'Все роли ещё загружаются — пока видны только работы из коллекции.',
    worksTitle: 'Роли',
    worksCount: roleCount,
    allInCollection: 'Все работы актёра уже в твоей коллекции.',
  },
  search: {
    title: 'Добавить актёра',
    subtitle:
      'Найди актёра по имени — покажем все его роли, даже те, что ещё не в коллекции.',
    placeholder: 'Например, Шаламе или Tom Hanks',
    label: 'Имя актёра',
    loading: 'Ищем актёров',
    emptyTitle: 'Найди актёра по имени',
    emptyDescription:
      'Здесь появятся актёры из твоей коллекции, на которых ты ещё не подписан.',
    hint: [
      'Тап по строке — профиль и все роли.',
      '＋ справа — отслеживать без перехода.',
    ],
  },
}

export const peopleSections: Record<PersonKind, PeopleSection> = {
  director: directorSection,
  actor: actorSection,
}
