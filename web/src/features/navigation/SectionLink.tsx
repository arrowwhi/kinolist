import type { Navigate } from '../../shared/lib/navigation'
import { haptic } from '../../shared/lib/telegram'
export function SectionLink({
  title,
  description,
  action,
  to,
  navigate,
}: {
  title: string
  description: string
  action: string
  to: string
  navigate: Navigate
}) {
  return (
    <a
      className="section-link"
      href={`#${to}`}
      onClick={(event) => {
        event.preventDefault()
        haptic('select')
        navigate(to)
      }}
    >
      <strong>{title}</strong>
      <span className="muted">{description}</span>
      <span className="section-link__action">{action} →</span>
    </a>
  )
}
