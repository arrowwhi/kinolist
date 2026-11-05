import { Button } from './Button'
export function EmptyState({
  title,
  description,
  action,
  onAction,
}: {
  title: string
  description?: string
  action?: string
  onAction?: () => void
}) {
  return (
    <div className="empty-state">
      <span className="empty-state__symbol" aria-hidden="true">
        ◷
      </span>
      <h2>{title}</h2>
      {description && <p className="muted">{description}</p>}
      {action && (
        <Button variant="secondary" onClick={onAction}>
          {action}
        </Button>
      )}
    </div>
  )
}
export function ErrorState({
  message,
  onRetry,
}: {
  message: string
  onRetry: () => void
}) {
  return (
    <div className="error-state" role="alert">
      <p>{message}</p>
      <Button variant="secondary" onClick={onRetry}>
        Повторить
      </Button>
    </div>
  )
}
export function InlineError({ message }: { message: string | null }) {
  return message ? (
    <p className="inline-error" role="alert">
      {message}
    </p>
  ) : null
}
export function SkeletonList() {
  return (
    <div
      className="skeleton-list"
      role="status"
      aria-label="Загружаем коллекцию"
    >
      {[1, 2, 3].map((i) => (
        <div className="skeleton-card" key={i}>
          <div className="skeleton skeleton-card__poster" />
          <div className="skeleton-card__copy">
            <div className="skeleton" />
            <div className="skeleton" />
            <div className="skeleton" />
          </div>
        </div>
      ))}
    </div>
  )
}
