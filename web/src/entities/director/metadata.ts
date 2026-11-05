export function directorCount(count: number) {
  const word =
    count % 100 >= 11 && count % 100 <= 14
      ? 'режиссёров'
      : count % 10 === 1
        ? 'режиссёр'
        : count % 10 >= 2 && count % 10 <= 4
          ? 'режиссёра'
          : 'режиссёров'
  return `${count} ${word}`
}
export function ratedWorks(count: number) {
  const single = count % 100 !== 11 && count % 10 === 1
  const few =
    !(count % 100 >= 11 && count % 100 <= 14) &&
    count % 10 >= 2 &&
    count % 10 <= 4
  return `${single ? 'Оценена' : 'Оценены'} ${count} ${single ? 'просмотренная работа' : few ? 'просмотренные работы' : 'просмотренных работ'}`
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
  const mod = count % 100
  const word =
    mod >= 11 && mod <= 14
      ? 'работ'
      : count % 10 === 1
        ? 'работа'
        : count % 10 >= 2 && count % 10 <= 4
          ? 'работы'
          : 'работ'
  return `${count} ${word}`
}
