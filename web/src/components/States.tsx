interface EmptyStateProps {
  icon: string
  title: string
  hint?: string
}

export function EmptyState({ icon, title, hint }: EmptyStateProps) {
  return (
    <div className="state">
      <div className="state__icon">{icon}</div>
      <div className="state__title">{title}</div>
      {hint && <div>{hint}</div>}
    </div>
  )
}

export function SkeletonList({ count = 4 }: { count?: number }) {
  return (
    <div aria-busy="true" aria-label="Загрузка">
      {Array.from({ length: count }, (_, i) => (
        <div className="skeleton" key={i} />
      ))}
    </div>
  )
}

export function Toast({ text }: { text: string }) {
  return (
    <div className="toast" role="status">
      {text}
    </div>
  )
}
