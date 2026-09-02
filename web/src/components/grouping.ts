import type { UserMovie } from '../api/types'

export type GroupBy = 'date' | 'genre' | 'none'

export interface Group {
  title: string
  items: UserMovie[]
}

const monthFormatter = new Intl.DateTimeFormat('ru-RU', {
  month: 'long',
  year: 'numeric',
})

function dateLabel(iso: string): string {
  const date = new Date(iso)
  const now = new Date()

  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const days = Math.floor((startOfToday.getTime() - date.getTime()) / 86_400_000)

  if (days < 0) return 'Сегодня'
  if (days === 0) return 'Сегодня'
  if (days === 1) return 'Вчера'
  if (days < 7) return 'На этой неделе'
  if (days < 30) return 'В этом месяце'

  return monthFormatter.format(date)
}

// groupMovies строит группы, сохраняя порядок, в котором пришёл список:
// сортировку решает сервер, клиент только расставляет заголовки.
export function groupMovies(items: UserMovie[], groupBy: GroupBy): Group[] {
  if (groupBy === 'none') {
    return [{ title: '', items }]
  }

  const groups = new Map<string, UserMovie[]>()

  for (const item of items) {
    const labels =
      groupBy === 'date'
        ? [dateLabel(item.added_at)]
        : item.movie.genres.length
          ? item.movie.genres.map((g) => g.name_ru)
          : ['Без жанра']

    for (const label of labels) {
      const bucket = groups.get(label)
      if (bucket) {
        bucket.push(item)
      } else {
        groups.set(label, [item])
      }
    }
  }

  const result = [...groups.entries()].map(([title, groupItems]) => ({
    title,
    items: groupItems,
  }))

  // Жанры сортируются по алфавиту; даты уже пришли в нужном порядке.
  if (groupBy === 'genre') {
    result.sort((a, b) => a.title.localeCompare(b.title, 'ru'))
  }

  return result
}
