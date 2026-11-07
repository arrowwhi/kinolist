import { useMemo } from 'react'
import { api } from '../../api/client'
import { useResource } from '../../shared/hooks/useResource'
import { contentCount, mediaFilterLabel } from '../../entities/movie/metadata'
import type { DirectorStats, Genre } from '../../api/types'
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
  director,
  genres,
  preferences,
  onPreferences,
  revision,
  navigate,
  back,
}: {
  status: CollectionStatus
  genre?: Genre
  director?: DirectorStats
  genres: Genre[]
  preferences: CollectionPreferences
  onPreferences: (value: CollectionPreferences) => void
  revision: number
  navigate: Navigate
  back: () => void
}) {
  const { filters, search } = preferences
  const directorIds = director
    ? [director.id]
    : filters.directors?.map((d) => d.id)
  const scoped = Boolean(genre || director)
  const genreResource = useResource(
    (signal) => api.collectionGenres(signal, filters.mediaType, directorIds),
    JSON.stringify([revision, filters.mediaType, directorIds]),
  )
  const genreStats = genreResource.data?.items ?? []
  const query = useDebounced(search)
  const collection = useCollection(
    status,
    filters,
    query,
    revision,
    genre?.id,
    director?.id,
  )
  const { page, summary, loading, error } = collection
  const groups = useMemo(
    () => groupMovies(page?.items ?? [], filters.groupBy, status, filters.sort),
    [page, filters.groupBy, status, filters.sort],
  )
  const watched = status === 'watched'
  const scope = director
    ? `/directors/${director.id}?status=${status}`
    : genre
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
    status === 'all'
      ? g.watched_count + g.want_to_watch_count > 0
      : watched
        ? g.watched_count > 0
        : g.want_to_watch_count > 0,
  )
  const genreLabel =
    genre?.name_ru ??
    (filters.genreIds.length
      ? genres
          .filter((g) => filters.genreIds.includes(g.id))
          .map((g) => g.name_ru)
          .join(', ')
      : 'все жанры')
  const filterLabel = [
    genreLabel,
    !director && filters.directors?.length
      ? filters.directors.map((d) => d.name).join(', ')
      : '',
  ]
    .filter(Boolean)
    .join(' · ')
  const statusPath = (value: CollectionStatus) =>
    `${director ? `/directors/${director.id}` : `/genres/${genre?.id}`}?status=${value}`
  const hasFilters = Boolean(
    filters.genreIds.length ||
    filters.mediaType ||
    (!director && filters.directors?.length),
  )
  return (
    <Shell
      title={
        director?.name ??
        genre?.name_ru ??
        (watched ? 'Просмотрено' : 'На потом')
      }
      subtitle={
        director
          ? 'Фильмы и сериалы из твоей коллекции.'
          : genre
            ? `Коллекция жанра «${genre.name_ru}»`
            : watched
              ? 'Истории, которые остались с тобой.'
              : 'Твоя следующая хорошая история.'
      }
      navigate={navigate}
      onBack={back}
      tab={
        director
          ? undefined
          : genre
            ? 'genres'
            : status === 'all'
              ? 'want_to_watch'
              : status
      }
    >
      <Button variant="secondary" onClick={openFilters}>
        Фильтры и сортировка
      </Button>
      {scoped ? (
        <div className="chips">
          <Chip
            active={status === 'want_to_watch'}
            onClick={() => navigate(statusPath('want_to_watch'), true)}
          >
            На потом
          </Chip>
          <Chip
            active={watched}
            onClick={() => navigate(statusPath('watched'), true)}
          >
            Просмотрено
          </Chip>
          <Chip
            active={status === 'all'}
            onClick={() => navigate(statusPath('all'), true)}
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
      {(watched || director) && summary && (
        <CollectionStats
          summary={summary}
          filterLabel={filterLabel}
          mediaType={filters.mediaType}
          showRatedCount={Boolean(director)}
        />
      )}
      {!watched && !director && (
        <p className="muted small collection-caption" aria-live="polite">
          По фильтрам · {mediaFilterLabel(filters.mediaType)} · {filterLabel}
          {summary
            ? ` · ${contentCount(summary.movie_count, filters.mediaType)}`
            : ''}
        </p>
      )}
      <Field
        type="search"
        aria-label={
          director ? 'Поиск среди работ режиссёра' : 'Поиск в коллекции'
        }
        placeholder={
          director
            ? 'Поиск среди работ режиссёра'
            : watched
              ? 'Поиск среди просмотренных'
              : 'Поиск в коллекции'
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
              : hasFilters || scoped
                ? 'По выбранным фильтрам пока ничего нет.'
                : watched
                  ? 'Отметь фильм или сериал просмотренным — он появится здесь.'
                  : 'Добавь фильм или сериал по названию.'
          }
          action={
            query || hasFilters
              ? 'Сбросить поиск и фильтры'
              : 'Добавить в коллекцию'
          }
          onAction={() =>
            query || hasFilters
              ? onPreferences({
                  filters: {
                    ...filters,
                    genreIds: [],
                    directors: [],
                    mediaType: undefined,
                  },
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
      {collection.hasMore && (
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
