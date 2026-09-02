import { useState } from 'react'

import { api, RequestError } from '../api/client'
import type { Status, UserMovie } from '../api/types'
import { haptic, webApp } from '../hooks/useTelegram'
import { Poster } from '../components/Poster'

interface MovieDetailProps {
  item: UserMovie
  onClose: () => void
  onChanged: (item: UserMovie) => void
  onDeleted: (id: number) => void
  onError: (message: string) => void
}

export function MovieDetail({
  item,
  onClose,
  onChanged,
  onDeleted,
  onError,
}: MovieDetailProps) {
  const [current, setCurrent] = useState(item)
  const [note, setNote] = useState(item.note ?? '')
  const [busy, setBusy] = useState(false)

  const { movie } = current

  async function patch(changes: Parameters<typeof api.updateMovie>[1]) {
    setBusy(true)
    try {
      const updated = await api.updateMovie(current.id, changes)
      setCurrent(updated)
      onChanged(updated)
      haptic('success')
    } catch (err) {
      haptic('error')
      onError(err instanceof RequestError ? err.message : 'Не удалось сохранить')
    } finally {
      setBusy(false)
    }
  }

  function setStatus(status: Status) {
    if (status === current.status) return
    haptic('select')
    void patch({ status })
  }

  function setRating(value: number) {
    haptic('light')
    // Повторное нажатие на ту же оценку снимает её.
    void patch({ user_rating: value === current.user_rating ? 0 : value })
  }

  function remove() {
    const confirmDelete = () => {
      setBusy(true)
      api
        .deleteMovie(current.id)
        .then(() => {
          haptic('success')
          onDeleted(current.id)
        })
        .catch((err) => {
          haptic('error')
          onError(err instanceof RequestError ? err.message : 'Не удалось удалить')
          setBusy(false)
        })
    }

    const app = webApp()
    if (app?.showConfirm) {
      app.showConfirm(`Удалить «${movie.title}» из коллекции?`, (ok) => {
        if (ok) confirmDelete()
      })
      return
    }
    if (window.confirm(`Удалить «${movie.title}» из коллекции?`)) {
      confirmDelete()
    }
  }

  return (
    <div className="sheet__backdrop" onClick={onClose} role="presentation">
      <div
        className="sheet"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label={movie.title}
      >
        <div className="sheet__grabber" />

        <div className="detail__hero">
          <Poster className="detail__poster" url={movie.poster_url} alt={movie.title} />
          <div>
            <h2 className="sheet__title">{movie.title}</h2>
            <p className="sheet__subtitle">
              {[
                movie.original_title !== movie.title ? movie.original_title : null,
                movie.year ? String(movie.year) : null,
                movie.runtime_min ? `${movie.runtime_min} мин` : null,
                movie.rating ? `⭐ ${movie.rating.toFixed(1)}` : null,
              ]
                .filter(Boolean)
                .join(' · ')}
            </p>
            {movie.genres.length > 0 && (
              <div className="genres">
                {movie.genres.map((g) => (
                  <span className="genre" key={g.id}>
                    {g.name_ru}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>

        {movie.overview && <p className="detail__overview">{movie.overview}</p>}

        <div className="sheet__section">
          <div className="sheet__label">Статус</div>
          <div className="segmented">
            <button
              type="button"
              aria-pressed={current.status === 'want_to_watch'}
              onClick={() => setStatus('want_to_watch')}
              disabled={busy}
            >
              Хочу посмотреть
            </button>
            <button
              type="button"
              aria-pressed={current.status === 'watched'}
              onClick={() => setStatus('watched')}
              disabled={busy}
            >
              Посмотрено
            </button>
          </div>
        </div>

        {current.status === 'watched' && (
          <div className="sheet__section">
            <div className="sheet__label">
              Моя оценка{current.user_rating ? `: ${current.user_rating}/10` : ''}
            </div>
            <div className="stars">
              {Array.from({ length: 10 }, (_, i) => i + 1).map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-label={`Оценка ${value}`}
                  aria-pressed={(current.user_rating ?? 0) >= value}
                  onClick={() => setRating(value)}
                  disabled={busy}
                >
                  ★
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="sheet__section">
          <div className="sheet__label">Заметка</div>
          <textarea
            className="field"
            rows={3}
            value={note}
            placeholder="Например, кто посоветовал"
            onChange={(e) => setNote(e.target.value)}
            onBlur={() => {
              if (note !== (current.note ?? '')) void patch({ note })
            }}
          />
        </div>

        {current.source_url && (
          <div className="sheet__section">
            <a className="sheet__label" href={current.source_url} target="_blank" rel="noreferrer">
              Исходный рилс ↗
            </a>
          </div>
        )}

        <div className="sheet__section">
          <button className="button button--danger" type="button" onClick={remove} disabled={busy}>
            Удалить из коллекции
          </button>
        </div>
      </div>
    </div>
  )
}
