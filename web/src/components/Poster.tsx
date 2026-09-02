interface PosterProps {
  url?: string
  alt: string
  className: string
}

export function Poster({ url, alt, className }: PosterProps) {
  if (!url) {
    return (
      <div className={`${className} card__poster--empty`} aria-hidden="true">
        🎬
      </div>
    )
  }

  return <img className={className} src={url} alt={alt} loading="lazy" />
}
