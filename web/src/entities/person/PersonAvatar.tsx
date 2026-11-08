import { useState } from 'react'
const colors = ['#38534a', '#5a4932', '#384f63', '#4b3a52', '#493c36']
// Аватар человека: фото из каталога или инициалы на цветной подложке.
export function PersonAvatar({
  name,
  url,
  size = 'md',
}: {
  name: string
  url?: string
  size?: 'sm' | 'md' | 'cast' | 'lg'
}) {
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  if (url && failedUrl !== url)
    return (
      <img
        className={`person-avatar person-avatar--${size}`}
        src={url}
        alt=""
        loading="lazy"
        onError={() => setFailedUrl(url)}
      />
    )
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0])
    .join('')
  const color =
    colors[
      Array.from(name).reduce((sum, letter) => sum + letter.charCodeAt(0), 0) %
        colors.length
    ]
  return (
    <span
      className={`person-avatar person-avatar--${size} person-avatar--initials`}
      style={{ backgroundColor: color }}
      aria-hidden="true"
    >
      {initials}
    </span>
  )
}
