import type { ButtonHTMLAttributes } from 'react'
export function Chip({
  active = false,
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { active?: boolean }) {
  return (
    <button
      {...props}
      type="button"
      className={`chip ${active ? 'chip--active' : ''}`}
      aria-pressed={active}
    >
      {children}
    </button>
  )
}
