import { useEffect, useRef, useState } from 'react'

import { api, RequestError } from '../api/client'
import type { Candidate, Job } from '../api/types'
import { haptic } from '../hooks/useTelegram'
import { Poster } from '../components/Poster'
import { EmptyState } from '../components/States'

interface AddMovieProps {
  onClose: () => void
  onAdded: () => void
  onError: (message: string) => void
}

// pollInterval — как часто спрашиваем статус задачи. Поиск занимает
// единицы секунд, чаще опрашивать незачем.
const pollInterval = 1200

export function AddMovie({ onClose, onAdded, onError }: AddMovieProps) {
  const [input, setInput] = useState('')
  const [job, setJob] = useState<Job | null>(null)
  const [busy, setBusy] = useState(false)
  const timer = useRef<number | undefined>(undefined)

  useEffect(() => () => window.clearTimeout(timer.current), [])

  // poll ждёт, пока воркер доведёт задачу до состояния, требующего решения.
  function poll(id: number) {
    timer.current = window.setTimeout(async () => {
      try {
        const next = await api.getJob(id)
        setJob(next)

        if (next.status === 'pending' || next.status === 'processing') {
          poll(id)
          return
        }
        setBusy(false)
        if (next.status === 'failed') haptic('error')
      } catch (err) {
        setBusy(false)
        onError(err instanceof RequestError ? err.message : 'Не удалось получить результат')
      }
    }, pollInterval)
  }

  async function submit() {
    const value = input.trim()
    if (!value || busy) return

    setBusy(true)
    haptic('light')

    try {
      const created = await api.createJob(value)
      setJob(created)
      poll(created.id)
    } catch (err) {
      setBusy(false)
      haptic('error')
      onError(err instanceof RequestError ? err.message : 'Не удалось начать поиск')
    }
  }

  async function confirm(candidate: Candidate) {
    if (!job) return

    setBusy(true)
    try {
      await api.confirmJob(job.id, candidate.provider, candidate.external_id)
      haptic('success')
      onAdded()
      onClose()
    } catch (err) {
      haptic('error')
      const message =
        err instanceof RequestError ? err.message : 'Не удалось добавить фильм'
      onError(message)
      // Дубль означает, что фильм уже есть — список стоит обновить.
      if (err instanceof RequestError && err.code === 'already_in_collection') {
        onAdded()
        onClose()
        return
      }
      setBusy(false)
    }
  }

  function reset() {
    if (job) void api.rejectJob(job.id).catch(() => undefined)
    setJob(null)
    setBusy(false)
    setInput('')
  }

  return (
    <div className="sheet__backdrop" onClick={onClose} role="presentation">
      <div
        className="sheet"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Добавить фильм"
      >
        <div className="sheet__grabber" />
        <h2 className="sheet__title">Добавить фильм</h2>
        <p className="sheet__subtitle">Ссылка на рилс или название фильма</p>

        {!job && (
          <>
            <textarea
              className="field"
              rows={2}
              value={input}
              autoFocus
              placeholder="https://instagram.com/reel/… или «Дюна»"
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  void submit()
                }
              }}
            />
            <div className="sheet__section">
              <button
                className="button"
                type="button"
                onClick={submit}
                disabled={busy || input.trim() === ''}
              >
                {busy ? 'Ищу…' : 'Найти'}
              </button>
            </div>
          </>
        )}

        {job && (job.status === 'pending' || job.status === 'processing') && (
          <div className="state">
            <div className="spinner" />
            <div className="state__title" style={{ marginTop: 14 }}>
              Ищу фильм…
            </div>
            <div>{job.kind === 'reel' ? 'Разбираю рилс' : 'Смотрю в кинобазах'}</div>
          </div>
        )}

        {job?.status === 'confirming' && (
          <>
            <div className="sheet__label" style={{ marginTop: 14 }}>
              {job.candidates.length === 1 ? 'Это тот фильм?' : 'Выберите фильм'}
            </div>
            {job.candidates.map((c) => (
              <button
                className="candidate"
                type="button"
                key={`${c.provider}-${c.external_id}`}
                onClick={() => confirm(c)}
                disabled={busy}
              >
                <Poster className="candidate__poster" url={c.poster_url} alt={c.title} />
                <div className="card__body">
                  <div className="card__title">{c.title}</div>
                  <div className="card__meta">
                    {c.year && <span>{c.year}</span>}
                    {c.rating ? <span>⭐ {c.rating.toFixed(1)}</span> : null}
                  </div>
                  {c.overview && <div className="card__overview">{c.overview}</div>}
                </div>
              </button>
            ))}
            <div className="sheet__section">
              <button className="button button--ghost" type="button" onClick={reset}>
                Ничего из этого
              </button>
            </div>
          </>
        )}

        {job?.status === 'failed' && (
          <>
            <EmptyState
              icon="😕"
              title={job.error ?? 'Не получилось'}
              hint="Попробуйте написать название текстом"
            />
            <button className="button button--ghost" type="button" onClick={reset}>
              Попробовать ещё раз
            </button>
          </>
        )}
      </div>
    </div>
  )
}
