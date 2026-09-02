import { useCallback, useEffect, useMemo, useState } from 'react'

import { api, RequestError } from '../api/client'
import type { Genre, Status, UserMovie } from '../api/types'
import { MovieCard } from '../components/MovieCard'
import { groupMovies, type GroupBy } from '../components/grouping'
import { EmptyState, SkeletonList, Toast } from '../components/States'
import { useDebounced } from '../hooks/useDebounced'
import { haptic } from '../hooks/useTelegram'
import { AddMovie } from './AddMovie'
import { MovieDetail } from './MovieDetail'

type StatusFilter = Status | 'all'

const groupLabels: Record<GroupBy, string> = {
  date: 'По дате',
  genre: 'По жанру',
  none: 'Без групп',
}

export function Collection() {
  const [items, setItems] = useState<UserMovie[]>([])
  const [total, setTotal] = useState(0)
  const [genres, setGenres] = useState<Genre[]>([])
  const [loading, setLoading] = useState(true)

  const [status, setStatus] = useState<StatusFilter>('all')
  const [genreID, setGenreID] = useState<number | null>(null)
  const [groupBy, setGroupBy] = useState<GroupBy>('date')
  const [search, setSearch] = useState('')
  const query = useDebounced(search)

  const [selected, setSelected] = useState<UserMovie | null>(null)
  const [adding, setAdding] = useState(false)
  const [toast, setToast] = useState<string | null>(null)

  const notify = useCallback((message: string) => {
    setToast(message)
    window.setTimeout(() => setToast(null), 2600)
  }, [])

  const load = useCallback(async () => {
    try {
      const page = await api.listMovies({
        status: status === 'all' ? undefined : status,
        genreIds: genreID ? [genreID] : undefined,
        query: query || undefined,
        sort: 'added_at',
        order: 'desc',
        limit: 200,
      })
      setItems(page.items)
      setTotal(page.total)
    } catch (err) {
      notify(err instanceof RequestError ? err.message : 'Не удалось загрузить коллекцию')
    } finally {
      setLoading(false)
    }
  }, [status, genreID, query, notify])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    api
      .genres()
      .then((r) => setGenres(r.items))
      .catch(() => undefined)
  }, [])

  // Жанры-фильтры показываем только те, что реально встречаются в коллекции:
  // полный справочник TMDB превратил бы строку фильтров в бесконечную ленту.
  const usedGenres = useMemo(() => {
    const present = new Set<number>()
    for (const item of items) {
      for (const g of item.movie.genres) present.add(g.id)
    }
    return genres.filter((g) => present.has(g.id) || g.id === genreID)
  }, [genres, items, genreID])

  const groups = useMemo(() => groupMovies(items, groupBy), [items, groupBy])

  function pickStatus(next: StatusFilter) {
    haptic('select')
    setStatus(next)
  }

  function pickGenre(id: number | null) {
    haptic('select')
    setGenreID(id)
  }

  function cycleGrouping() {
    haptic('select')
    setGroupBy((prev) => (prev === 'date' ? 'genre' : prev === 'genre' ? 'none' : 'date'))
  }

  const isFiltered = status !== 'all' || genreID !== null || query !== ''

  return (
    <div className="app">
      <header className="header">
        <h1 className="header__title">
          Коллекция
          {total > 0 && <span className="header__count">{total}</span>}
        </h1>

        <label className="search">
          <span aria-hidden="true">🔍</span>
          <input
            type="search"
            value={search}
            placeholder="Поиск по названию"
            onChange={(e) => setSearch(e.target.value)}
          />
        </label>

        <div className="chips">
          <button
            type="button"
            className={`chip ${status === 'all' ? 'chip--active' : ''}`}
            onClick={() => pickStatus('all')}
          >
            Все
          </button>
          <button
            type="button"
            className={`chip ${status === 'want_to_watch' ? 'chip--active' : ''}`}
            onClick={() => pickStatus('want_to_watch')}
          >
            Хочу посмотреть
          </button>
          <button
            type="button"
            className={`chip ${status === 'watched' ? 'chip--active' : ''}`}
            onClick={() => pickStatus('watched')}
          >
            Посмотрено
          </button>
          <button type="button" className="chip" onClick={cycleGrouping}>
            {groupLabels[groupBy]}
          </button>
        </div>

        {usedGenres.length > 0 && (
          <div className="chips">
            <button
              type="button"
              className={`chip ${genreID === null ? 'chip--active' : ''}`}
              onClick={() => pickGenre(null)}
            >
              Все жанры
            </button>
            {usedGenres.map((g) => (
              <button
                type="button"
                key={g.id}
                className={`chip ${genreID === g.id ? 'chip--active' : ''}`}
                onClick={() => pickGenre(genreID === g.id ? null : g.id)}
              >
                {g.name_ru}
              </button>
            ))}
          </div>
        )}
      </header>

      <main className="content">
        {loading && <SkeletonList />}

        {!loading && items.length === 0 && !isFiltered && (
          <EmptyState
            icon="🍿"
            title="Пока пусто"
            hint="Пришлите боту ссылку на рилс или добавьте фильм по названию"
          />
        )}

        {!loading && items.length === 0 && isFiltered && (
          <EmptyState icon="🔍" title="Ничего не найдено" hint="Попробуйте изменить фильтры" />
        )}

        {!loading &&
          groups.map((group) => (
            <section className="group" key={group.title || 'all'}>
              {group.title && <h2 className="group__title">{group.title}</h2>}
              {group.items.map((item) => (
                <MovieCard key={`${group.title}-${item.id}`} item={item} onOpen={setSelected} />
              ))}
            </section>
          ))}
      </main>

      <button className="fab" type="button" onClick={() => setAdding(true)}>
        <span aria-hidden="true">＋</span> Добавить
      </button>

      {selected && (
        <MovieDetail
          item={selected}
          onClose={() => setSelected(null)}
          onChanged={(updated) =>
            setItems((prev) => prev.map((i) => (i.id === updated.id ? updated : i)))
          }
          onDeleted={(id) => {
            setItems((prev) => prev.filter((i) => i.id !== id))
            setTotal((prev) => Math.max(0, prev - 1))
            setSelected(null)
            notify('Фильм удалён')
          }}
          onError={notify}
        />
      )}

      {adding && (
        <AddMovie
          onClose={() => setAdding(false)}
          onAdded={() => {
            void load()
            notify('Фильм добавлен')
          }}
          onError={notify}
        />
      )}

      {toast && <Toast text={toast} />}
    </div>
  )
}
