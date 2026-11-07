import { plural } from '../../shared/lib/format.ts'
export function directorCount(count: number) {
  return `${count} ${plural(count, ['режиссёр', 'режиссёра', 'режиссёров'])}`
}
export function ratedWorks(count: number) {
  const verb = plural(count, ['Оценена', 'Оценены', 'Оценены'])
  const works = plural(count, [
    'просмотренная работа',
    'просмотренные работы',
    'просмотренных работ',
  ])
  return `${verb} ${count} ${works}`
}
export function personalAverage(value: number | null) {
  return value == null
    ? '—'
    : value.toLocaleString('ru-RU', {
        minimumFractionDigits: 1,
        maximumFractionDigits: 1,
      })
}

export function workCount(count: number) {
  return `${count} ${plural(count, ['работа', 'работы', 'работ'])}`
}
