import { useState } from 'react'
const colors = ['#38534a', '#5a4932', '#384f63', '#4b3a52', '#493c36']
export function DirectorAvatar({
  name,
  url,
  size = 'md',
}: {
  name: string
  url?: string
  size?: 'sm' | 'md' | 'lg'
}) {
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  if (url && failedUrl !== url)
    return (
      <img
        className={`director-avatar director-avatar--${size}`}
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
      className={`director-avatar director-avatar--${size} director-avatar--initials`}
      style={{ backgroundColor: color }}
      aria-hidden="true"
    >
      {initials}
    </span>
  )
}
