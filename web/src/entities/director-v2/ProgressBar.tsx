export function ProgressBar({
  value,
  label,
}: {
  value: number
  label: string
}) {
  return (
    <span
      className="director-progress"
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={value}
    >
      <span style={{ width: `${value}%` }} />
    </span>
  )
}
