import { useState } from 'react'
import { Button } from '../../shared/ui/Button'
import { TextArea } from '../../shared/ui/Field'
import { InlineError } from '../../shared/ui/Feedback'
import { haptic } from '../../shared/lib/telegram'
export function RatingForm({
  title,
  initialRating = 0,
  initialReview = '',
  busy,
  error,
  onSave,
}: {
  title: string
  initialRating?: number
  initialReview?: string
  busy: boolean
  error: string | null
  onSave: (rating: number, review: string) => void
}) {
  const [rating, setRating] = useState(initialRating)
  const [review, setReview] = useState(initialReview)
  const labels = [
    'Без оценки',
    'Совсем не понравился',
    'Не понравился',
    'Скорее не понравился',
    'Могло быть лучше',
    'На один раз',
    'Неплохо',
    'Понравилось',
    'Очень хороший',
    'Очень понравился',
    'Один из любимых',
  ]
  return (
    <form
      className="stack rating-form"
      onSubmit={(e) => {
        e.preventDefault()
        onSave(rating, review)
      }}
    >
      <p className="muted">{title}</p>
      <h2>Твоя оценка</h2>
      <div className="rating-summary">
        <strong>{rating || '—'}</strong>
        <span className="muted">
          / 10
          <br />
          {labels[rating]}
        </span>
      </div>
      <div
        className="rating-scale"
        role="radiogroup"
        aria-label="Оценка от 1 до 10"
        onKeyDown={(e) => {
          if (
            busy ||
            ![
              'ArrowLeft',
              'ArrowRight',
              'ArrowUp',
              'ArrowDown',
              'Home',
              'End',
            ].includes(e.key)
          )
            return
          e.preventDefault()
          const value =
            e.key === 'Home'
              ? 1
              : e.key === 'End'
                ? 10
                : (((rating || 1) -
                    1 +
                    (e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? 9 : 1)) %
                    10) +
                  1
          setRating(value)
          e.currentTarget
            .querySelectorAll<HTMLButtonElement>('button')
            [value - 1]?.focus()
        }}
      >
        {Array.from({ length: 10 }, (_, i) => i + 1).map((value) => (
          <button
            key={value}
            type="button"
            role="radio"
            aria-checked={rating === value}
            tabIndex={rating === value || (!rating && value === 1) ? 0 : -1}
            aria-label={`${value} из 10`}
            className={rating === value ? 'is-active' : ''}
            disabled={busy}
            onClick={() => {
              setRating(value)
              haptic('select')
            }}
          >
            {value}
          </button>
        ))}
      </div>
      <TextArea
        label="Твой отзыв"
        placeholder="Что запомнилось? Какие впечатления остались?"
        rows={5}
        maxLength={5000}
        value={review}
        onChange={(e) => setReview(e.target.value)}
        hint={`${review.length}/5000 · Необязательно`}
        disabled={busy}
      />
      <p className="muted small">Оценку и отзыв можно изменить позже.</p>
      <InlineError message={error} />
      <div className="form-actions">
        <Button type="submit" busy={busy}>
          Сохранить просмотр
        </Button>
        <Button
          variant="secondary"
          disabled={busy}
          onClick={() => onSave(0, review)}
        >
          Отметить без оценки
        </Button>
      </div>
    </form>
  )
}
