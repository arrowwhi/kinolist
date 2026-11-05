import { useEffect, useRef, useState } from 'react'
import { api, RequestError } from '../../api/client'
import type {
  Candidate,
  ConfirmOptions,
  Job,
  Movie,
  UserMovie,
} from '../../api/types'
import { errorMessage } from '../../shared/lib/errors'
import { haptic } from '../../shared/lib/telegram'
export function useAddMovie(
  onAdded: (item: UserMovie) => void,
  onExisting: (id: number) => void,
) {
  const [input, setInput] = useState('')
  const [preferredCandidate, setPreferredCandidate] =
    useState<Candidate | null>(null)
  const [job, setJob] = useState<Job | null>(null)
  const [candidate, setCandidate] = useState<Candidate | null>(null)
  const [movie, setMovie] = useState<Movie | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [pollAttempt, setPollAttempt] = useState(0)
  const [pollStopped, setPollStopped] = useState(false)
  const [existingId, setExistingId] = useState<number | null>(null)
  const alive = useRef(true)
  const lock = useRef(false)
  const selection = useRef(0)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
      selection.current++
    }
  }, [])
  useEffect(() => {
    if (!job || !['pending', 'processing'].includes(job.status)) return
    const controller = new AbortController()
    let timer: number | undefined
    let attempts = 0
    setPollStopped(false)
    const poll = async () => {
      try {
        const next = await api.getJob(job.id, controller.signal)
        if (controller.signal.aborted) return
        setJob(next)
        if (next.status === 'failed' || next.status === 'cancelled') {
          setError(
            next.error ||
              'Поиск завершён без результата. Попробуй другое название.',
          )
          return
        }
        if (next.status === 'done') {
          setError('Эта задача уже завершена. Начни новый поиск.')
          return
        }
        if (next.status === 'pending' || next.status === 'processing') {
          if (++attempts >= 100) {
            setError(
              'Поиск занимает больше времени. Можно продолжить ожидание.',
            )
            setPollStopped(true)
            return
          }
          timer = window.setTimeout(poll, 1200)
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(errorMessage(err))
          setPollStopped(true)
        }
      }
    }
    timer = window.setTimeout(poll, 600)
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [job?.id, job?.status, pollAttempt])
  async function search(inputOverride?: string, preferred?: Candidate) {
    const query = (inputOverride ?? input).trim()
    if (!query || lock.current) return
    lock.current = true
    setBusy(true)
    setError(null)
    setExistingId(null)
    try {
      if (job && ['pending', 'processing', 'confirming'].includes(job.status))
        await api.rejectJob(job.id)
      if (!alive.current) return
      selection.current++
      setJob(null)
      setCandidate(null)
      setMovie(null)
      setInput(query)
      setPreferredCandidate(preferred ?? null)
      const created = await api.createJob(query)
      if (alive.current) setJob(created)
    } catch (err) {
      if (alive.current) setError(errorMessage(err))
    } finally {
      lock.current = false
      if (alive.current) setBusy(false)
    }
  }
  async function select(value: Candidate) {
    if (!job || lock.current) return
    const version = ++selection.current
    setExistingId(null)
    setCandidate(value)
    setMovie(null)
    setBusy(true)
    setError(null)
    try {
      const details = await api.candidateDetails(job.id, value)
      if (alive.current && version === selection.current) setMovie(details)
    } catch (err) {
      if (alive.current && version === selection.current)
        setError(errorMessage(err, 'Не удалось загрузить карточку'))
    } finally {
      if (alive.current && version === selection.current) setBusy(false)
    }
  }
  async function confirm(options: ConfirmOptions = {}) {
    if (!job || !candidate || lock.current) return
    lock.current = true
    setBusy(true)
    setError(null)
    try {
      const result = await api.confirmJob(
        job.id,
        candidate.provider,
        candidate.external_id,
        { ...options, media_type: candidate.media_type ?? 'movie' },
      )
      if (alive.current) {
        haptic('success')
        onAdded(result)
      }
    } catch (err) {
      if (alive.current) {
        haptic('error')
        setError(errorMessage(err))
        if (err instanceof RequestError && err.existingUserMovieId)
          setExistingId(err.existingUserMovieId)
      }
    } finally {
      lock.current = false
      if (alive.current) setBusy(false)
    }
  }
  async function reset() {
    if (lock.current) return
    lock.current = true
    setBusy(true)
    setError(null)
    try {
      if (job && ['pending', 'processing', 'confirming'].includes(job.status))
        await api.rejectJob(job.id)
      if (alive.current) {
        selection.current++
        setJob(null)
        setPreferredCandidate(null)
        setMovie(null)
        setCandidate(null)
        setExistingId(null)
        setPollStopped(false)
      }
    } catch (err) {
      if (alive.current) setError(errorMessage(err))
    } finally {
      lock.current = false
      if (alive.current) setBusy(false)
    }
  }
  function clearSelection() {
    setExistingId(null)
    selection.current++
    setMovie(null)
    setCandidate(null)
    setBusy(false)
    setError(null)
  }
  return {
    input,
    setInput,
    preferredCandidate,
    job,
    candidate,
    movie,
    busy,
    error,
    setError,
    existingId,
    search,
    select,
    confirm,
    reset,
    clearSelection,
    pollStopped,
    resume: () => {
      setError(null)
      setPollAttempt((v) => v + 1)
    },
    openExisting: () => {
      if (existingId) onExisting(existingId)
    },
  }
}
