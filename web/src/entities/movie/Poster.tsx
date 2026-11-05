import { useState } from 'react'
const colors = ['#5a4932', '#38534a', '#384f63', '#493c36']
export function Poster({
  url,
  title,
  size = 'card',
}: {
  url?: string
  title: string
  size?: 'card' | 'hero' | 'wide'
}) {
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  if (url && failedUrl !== url)
    return (
      <img
        className={`poster poster--${size}`}
        src={url}
        alt={`Постер «${title}»`}
        loading="lazy"
        onError={() => setFailedUrl(url)}
      />
    )
  const color =
    colors[
      Array.from(title).reduce((sum, letter) => sum + letter.charCodeAt(0), 0) %
        colors.length
    ]
  return (
    <div
      className={`poster poster--${size} poster--art`}
      style={{ backgroundColor: color }}
      aria-label={`Обложка «${title}»`}
    >
      <span className="poster__eyebrow">К И Н О</span>
      <span className="poster__title">{title}</span>
    </div>
  )
}
