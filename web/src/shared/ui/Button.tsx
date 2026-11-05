import type { ButtonHTMLAttributes } from 'react'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'quiet' | 'danger'
  busy?: boolean
}
export function Button({
  variant = 'primary',
  busy,
  disabled,
  children,
  className = '',
  ...props
}: Props) {
  return (
    <button
      {...props}
      type={props.type ?? 'button'}
      className={`button button--${variant} ${className}`}
      disabled={disabled || busy}
      aria-busy={busy || undefined}
    >
      {busy && <span className="spinner" aria-hidden="true" />}
      {children}
    </button>
  )
}
