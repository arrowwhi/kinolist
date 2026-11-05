import { api } from '../../api/client'
import { useResource } from '../../shared/hooks/useResource'
import { ErrorState } from '../../shared/ui/Feedback'
export function GlobalStatistics({ revision }: { revision: number }) {
  const { data, error, loading, retry } = useResource(
    (signal) => api.statistics(signal),
    revision,
  )
  return (
    <section className="stats-block" aria-label="Общая статистика коллекции">
      <h2 className="section-label">Вся коллекция</h2>
      {loading ? (
        <div
          className="global-statistics skeleton"
          role="status"
          aria-label="Загружаем общую статистику"
        />
      ) : error ? (
        <ErrorState message={error} onRetry={retry} />
      ) : data ? (
        <dl className="global-statistics">
          {[
            { count: data.total_count, label: 'всего' },
            { count: data.want_to_watch_count, label: 'на потом' },
            { count: data.watched_count, label: 'просмотрено' },
          ].map((item) => (
            <div key={item.label}>
              <dt>{item.label}</dt>
              <dd>{new Intl.NumberFormat('ru-RU').format(item.count)}</dd>
            </div>
          ))}
        </dl>
      ) : null}
      <p className="statistics-caption muted">Фильмы и сериалы вместе</p>
    </section>
  )
}
