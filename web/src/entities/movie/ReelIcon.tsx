/** Пиктограмма рилса: кнопка «play» в скруглённом квадрате. */
export function ReelIcon({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  return (
    <span className={`reel-icon reel-icon--${size}`} aria-hidden="true">
      <svg viewBox="0 0 12 14" focusable="false">
        <path d="M0 0 L12 7 L0 14 Z" />
      </svg>
    </span>
  )
}
