import { api } from '../../api/client'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { useResource } from '../../shared/hooks/useResource'
import { EmptyState, ErrorState, SkeletonList } from '../../shared/ui/Feedback'
export function GenresPage({
  navigate,
  revision,
  back,
}: {
  navigate: Navigate
  revision: number
  back: () => void
}) {
  const { data, error, loading, retry } = useResource(
    (signal) => api.collectionGenres(signal),
    revision,
  )
  return (
    <Shell
      title="Жанры"
      subtitle="Выбирай кино под настроение."
      navigate={navigate}
      tab="genres"
      onBack={back}
      themeToggle
    >
      {loading ? (
        <SkeletonList />
      ) : error ? (
        <ErrorState message={error} onRetry={retry} />
      ) : !data?.items.length ? (
        <EmptyState
          title="Здесь будут твои жанры"
          description="Добавь фильм или сериал — его жанры появятся здесь."
          action="Добавить в коллекцию"
          onAction={() => navigate('/add')}
        />
      ) : (
        <div className="genre-grid">
          {data.items.map((g, i) => (
            <button
              type="button"
              key={g.id}
              className={`genre-tile genre-tile--${i % 3}`}
              onClick={() => navigate(`/genres/${g.id}`)}
            >
              <strong>{g.name_ru}</strong>
              <span>
                {g.want_to_watch_count} на потом · {g.watched_count} просмотрено
              </span>
              <span className="accent">Открыть коллекцию →</span>
            </button>
          ))}
        </div>
      )}
    </Shell>
  )
}
