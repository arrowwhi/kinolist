import { useMemo } from 'react'
import type { Genre, GenreStats } from '../../api/types'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { Button } from '../../shared/ui/Button'
import { Chip } from '../../shared/ui/Chip'
import { Field } from '../../shared/ui/Field'
import { EmptyState, ErrorState, SkeletonList } from '../../shared/ui/Feedback'
import { useDebounced } from '../../shared/hooks/useDebounced'
import { useCollection } from '../../features/collection/useCollection'
import { CollectionStats } from '../../features/collection/CollectionStats'
import { groupMovies } from '../../features/collection/model'
import type {
  CollectionFilters,
  CollectionStatus,
} from '../../features/collection/model'
import { MovieCard } from '../../entities/movie/MovieCard'
export interface CollectionPreferences {
  filters: CollectionFilters
  search: string
}
export function CollectionPage({
  status,
  genre,
  genres,
  genreStats,
  preferences,
  onPreferences,
  revision,
  navigate,
  back,
}: {
  status: CollectionStatus
  genre?: Genre
  genres: Genre[]
  genreStats: GenreStats[]
  preferences: CollectionPreferences
  onPreferences: (value: CollectionPreferences) => void
  revision: number
  navigate: Navigate
  back: () => void
}) {
  const { filters, search } = preferences
  const query = useDebounced(search)
  const collection = useCollection(status, filters, query, revision, genre?.id)
  const { page, summary, loading, error } = collection
  const groups = useMemo(
    () => groupMovies(page?.items ?? [], filters.groupBy, status),
    [page, filters.groupBy, status],
  )
  const watched = status === 'watched'
  const scope = genre
    ? `/genres/${genre.id}?status=${status}`
    : `/collection/${status}`
  const openFilters = () =>
    navigate(`/filters?return=${encodeURIComponent(scope)}`)
  const chooseGenre = (id: number | null) =>
    onPreferences({
      ...preferences,
      filters: { ...filters, genreIds: id == null ? [] : [id] },
    })
  const usedGenres = genreStats.filter((g) =>
    watched ? g.watched_count > 0 : g.want_to_watch_count > 0,
  )
  const filterLabel =
    genre?.name_ru ??
    (filters.genreIds.length
      ? genres
          .filter((g) => filters.genreIds.includes(g.id))
          .map((g) => g.name_ru)
          .join(', ')
      : 'все жанры')
  return (
    <Shell
      title={genre?.name_ru ?? (watched ? 'Просмотрено' : 'На потом')}
      subtitle={
        genre
          ? `Коллекция жанра «${genre.name_ru}»`
          : watched
            ? 'Истории, которые остались с тобой.'
            : 'Твоя следующая хорошая история.'
      }
      navigate={navigate}
      onBack={genre ? back : undefined}
      tab={genre ? 'genres' : status === 'all' ? 'want_to_watch' : status}
      actions={
        !watched ? (
          <Button onClick={() => navigate('/add')}>＋ Добавить фильм</Button>
        ) : undefined
      }
    >
      <Button variant="secondary" onClick={openFilters}>
        Фильтры и сортировка
      </Button>
      {genre ? (
        <div className="chips">
          <Chip
            active={status === 'want_to_watch'}
            onClick={() =>
              navigate(`/genres/${genre.id}?status=want_to_watch`, true)
            }
          >
            На потом
          </Chip>
          <Chip
            active={watched}
            onClick={() => navigate(`/genres/${genre.id}?status=watched`, true)}
          >
            Просмотрено
          </Chip>
          <Chip
            active={status === 'all'}
            onClick={() => navigate(`/genres/${genre.id}?status=all`, true)}
          >
            Все
          </Chip>
        </div>
      ) : watched ? (
        <div className="chips">
          <Chip
            active={filters.sort === 'watched_at'}
            onClick={() =>
              onPreferences({
                ...preferences,
                filters: { ...filters, sort: 'watched_at', order: 'desc' },
              })
            }
          >
            По дате
          </Chip>
          <Chip
            active={filters.sort === 'user_rating'}
            onClick={() =>
              onPreferences({
                ...preferences,
                filters: { ...filters, sort: 'user_rating', order: 'desc' },
              })
            }
          >
            По оценке
          </Chip>
          <Chip active={filters.genreIds.length > 0} onClick={openFilters}>
            Жанр{filters.genreIds.length ? ` · ${filters.genreIds.length}` : ''}
          </Chip>
        </div>
      ) : (
        <div className="chips">
          <Chip
            active={!filters.genreIds.length}
            onClick={() => chooseGenre(null)}
          >
            Все
            {summary && !filters.genreIds.length
              ? ` ${summary.movie_count}`
              : ''}
          </Chip>
          {usedGenres.slice(0, 5).map((g) => (
            <Chip
              key={g.id}
              active={filters.genreIds.includes(g.id)}
              onClick={() =>
                chooseGenre(filters.genreIds.includes(g.id) ? null : g.id)
              }
            >
              {g.name_ru}
            </Chip>
          ))}
        </div>
      )}
      {watched && summary && (
        <CollectionStats summary={summary} filterLabel={filterLabel} />
      )}
      <Field
        type="search"
        aria-label="Поиск фильмов"
        placeholder={
          watched ? 'Поиск среди просмотренных' : 'Поиск в коллекции'
        }
        value={search}
        onChange={(e) =>
          onPreferences({ ...preferences, search: e.target.value })
        }
      />
      {loading ? (
        <SkeletonList />
      ) : error && !page ? (
        <ErrorState message={error} onRetry={collection.retry} />
      ) : page?.items.length === 0 ? (
        <EmptyState
          title={
            query
              ? 'Ничего не найдено'
              : watched
                ? 'Истории ещё впереди'
                : 'Хорошее кино начинается здесь'
          }
          description={
            query
              ? 'Попробуй другое название или измени фильтры.'
              : filters.genreIds.length || genre
                ? 'В выбранном жанре пока нет фильмов.'
                : watched
                  ? 'Отметь первый просмотренный фильм — он появится здесь.'
                  : 'Добавь название или ссылку на рилс. Сохраним фильм на потом.'
          }
          action={
            query || filters.genreIds.length
              ? 'Сбросить поиск и фильтры'
              : 'Добавить фильм'
          }
          onAction={() =>
            query || filters.genreIds.length
              ? onPreferences({
                  filters: { ...filters, genreIds: [] },
                  search: '',
                })
              : navigate('/add')
          }
        />
      ) : (
        <div className="movie-groups">
          {groups.map((group) => (
            <section key={group.title || 'all'} className="movie-group">
              {group.title && <h2 className="section-label">{group.title}</h2>}
              {group.items.map((item) => (
                <MovieCard
                  key={item.id}
                  item={item}
                  onOpen={(id) => navigate(`/movie/${id}`)}
                />
              ))}
            </section>
          ))}
        </div>
      )}
      {error && page && (
        <ErrorState message={error} onRetry={collection.loadMore} />
      )}
      {page && page.items.length < page.total && (
        <Button
          variant="secondary"
          busy={collection.loadingMore}
          onClick={collection.loadMore}
        >
          Показать ещё
        </Button>
      )}
    </Shell>
  )
}
