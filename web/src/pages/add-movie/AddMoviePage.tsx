import { useEffect, useId, useState } from 'react'
import type { Candidate, UserMovie } from '../../api/types'
import type { Navigate } from '../../shared/lib/navigation'
import type { RecentCandidate } from '../../features/add-movie/useRecentCandidates'
import { Shell } from '../../widgets/app-shell/Shell'
import { readClipboard } from '../../shared/lib/telegram'
import { useAddMovie } from '../../features/add-movie/useAddMovie'
import { RatingForm } from '../../features/rating/RatingForm'
import { Button } from '../../shared/ui/Button'
import { Field } from '../../shared/ui/Field'
import { EmptyState, InlineError, SkeletonList } from '../../shared/ui/Feedback'
import { MoviePreview } from '../../entities/movie/MoviePreview'
import { CandidateCard } from '../../entities/movie/CandidateCard'
import { candidateKey, movieMetadata } from '../../entities/movie/metadata'
import { ReelIcon } from '../../entities/movie/ReelIcon'
import {
  CaptionPatterns,
  ReelHint,
  ReelLink,
  ReelSource,
} from '../../features/add-movie/ReelHint'
export function AddMoviePage({
  navigate,
  back,
  changed,
  recent,
  remember,
}: {
  navigate: Navigate
  back: () => void
  changed: (message: string) => void
  recent: RecentCandidate[]
  remember: (input: string, candidates: Candidate[]) => void
}) {
  const queryId = useId()
  const onAdded = (item: UserMovie) => {
    changed(
      item.status === 'watched' ? 'Просмотр сохранён' : 'Добавлено в коллекцию',
    )
    navigate(`/movie/${item.id}`, true)
  }
  const flow = useAddMovie(onAdded, (id) => {
    changed('Уже в коллекции')
    navigate(`/movie/${id}`, true)
  })
  const [rating, setRating] = useState(false)
  // Дубль обнаружился уже на форме оценки — возвращаемся к карточке,
  // где есть кнопка «Открыть в коллекции».
  useEffect(() => {
    if (flow.existingId) setRating(false)
  }, [flow.existingId])
  const candidates =
    flow.job?.status === 'confirming' ? flow.job.candidates : []
  useEffect(() => {
    if (!candidates.length) return
    remember(flow.job?.query_title || flow.input, candidates)
    const preferred = flow.preferredCandidate
    const chosen = preferred
      ? candidates.find((c) => candidateKey(c) === candidateKey(preferred))
      : candidates.length === 1
        ? candidates[0]
        : null
    if (chosen && !flow.candidate) void flow.select(chosen)
    // A new job owns a fresh candidate set. Don't reselect after manual Back.
  }, [flow.job?.id, flow.job?.status, candidates.length, remember])
  useEffect(() => {
    if (flow.movie && flow.candidate)
      remember(flow.job?.query_title || flow.input, [
        { ...flow.candidate, ...flow.movie },
      ])
  }, [flow.movie, remember])
  const waiting =
    !!flow.job && ['pending', 'processing'].includes(flow.job.status)
  const reel = flow.job?.kind === 'reel'
  // Запрос, который сервер достал из описания рилса или взял из поля ввода.
  const query = flow.job?.query_title || flow.input
  const querySource =
    reel && flow.job?.query_title ? (
      <ReelSource title={flow.job.query_title} year={flow.job.query_year} />
    ) : null
  const tv = (flow.movie?.media_type ?? flow.candidate?.media_type) === 'tv'
  async function goBack() {
    if (flow.busy) return
    if (rating) {
      setRating(false)
      return
    }
    if (flow.candidate && candidates.length > 1) {
      flow.clearSelection()
      return
    }
    if (flow.job) {
      await flow.reset()
      return
    }
    back()
  }
  // Из рилса название не достали — возвращаем к форме с тем, что удалось
  // понять (или с пустым полем), и сразу ставим курсор.
  async function enterTitle(prefill: string) {
    await flow.reset()
    flow.setInput(prefill)
    requestAnimationFrame(() => document.getElementById(queryId)?.focus())
  }
  async function paste() {
    try {
      flow.setInput(await readClipboard())
    } catch {
      flow.setError(
        'Не удалось прочитать буфер. Введи название в поле вручную.',
      )
    }
  }
  return (
    <Shell
      title={
        rating
          ? tv
            ? 'Как тебе сериал?'
            : 'Как тебе фильм?'
          : flow.candidate
            ? tv
              ? 'Это тот сериал?'
              : 'Это тот фильм?'
            : waiting
              ? 'Ищем твоё кино'
              : 'Добавить в коллекцию'
      }
      navigate={navigate}
      onBack={() => void goBack()}
    >
      {rating && flow.movie ? (
        <RatingForm
          title={`${flow.movie.title} · ${movieMetadata(flow.movie)}`}
          busy={flow.busy}
          error={flow.error}
          onSave={(value, review) =>
            void flow.confirm({ status: 'watched', user_rating: value, review })
          }
        />
      ) : !flow.job || (candidates.length > 0 && !flow.candidate) ? (
        <>
          <form
            className="stack add-form"
            onSubmit={(e) => {
              e.preventDefault()
              void flow.search()
            }}
          >
            <p className="muted">
              Увидел интересное? Сохрани сейчас,
              <br />
              посмотри, когда захочется.
            </p>
            <div className="field-label">
              <label className="add-form__label" htmlFor={queryId}>
                Название или ссылка
              </label>
              <div className="add-form__row">
                <Field
                  id={queryId}
                  placeholder="«Идеальные дни»"
                  value={flow.input}
                  onChange={(e) => flow.setInput(e.target.value)}
                  disabled={flow.busy}
                  required
                  maxLength={2000}
                  autoComplete="off"
                />
                <Button
                  variant="secondary"
                  className="add-form__paste"
                  onClick={() => void paste()}
                  disabled={flow.busy}
                >
                  Вставить
                </Button>
              </div>
            </div>
            <InlineError message={flow.error} />
            <Button
              type="submit"
              disabled={!flow.input.trim()}
              busy={flow.busy}
            >
              Найти
            </Button>
          </form>
          {!candidates.length && <ReelHint />}
          {candidates.length ? (
            <section className="stack" aria-label="Результаты поиска">
              <h2>Результаты поиска</h2>
              {querySource ?? (
                <p className="muted small">По запросу «{query}»</p>
              )}
              {candidates.map((c) => (
                <CandidateCard
                  key={candidateKey(c)}
                  candidate={c}
                  disabled={flow.busy}
                  onSelect={() => void flow.select(c)}
                />
              ))}
            </section>
          ) : recent.length > 0 ? (
            <section className="stack" aria-label="Недавние результаты">
              <h2>Недавние результаты</h2>
              {recent.slice(0, 3).map((entry) => (
                <CandidateCard
                  key={candidateKey(entry.candidate)}
                  candidate={entry.candidate}
                  disabled={flow.busy}
                  onSelect={() =>
                    void flow.search(entry.input, entry.candidate)
                  }
                />
              ))}
            </section>
          ) : null}
        </>
      ) : flow.candidate ? (
        <>
          {querySource ?? <p className="muted">По запросу «{query}»</p>}
          {flow.busy && !flow.movie ? (
            <SkeletonList />
          ) : flow.movie ? (
            <MoviePreview movie={flow.movie} />
          ) : null}
          <InlineError message={flow.error} />
          {flow.existingId ? (
            <Button onClick={flow.openExisting}>Открыть в коллекции</Button>
          ) : flow.movie ? (
            <div className="stack">
              <Button busy={flow.busy} onClick={() => void flow.confirm()}>
                ＋ Сохранить на потом
              </Button>
              <Button
                variant="secondary"
                disabled={flow.busy}
                onClick={() => setRating(true)}
              >
                Уже смотрел
              </Button>
            </div>
          ) : !flow.busy ? (
            <Button onClick={() => void flow.select(flow.candidate!)}>
              Повторить загрузку карточки
            </Button>
          ) : null}
          <Button
            variant="quiet"
            disabled={flow.busy}
            onClick={() =>
              candidates.length > 1 ? flow.clearSelection() : void flow.reset()
            }
          >
            {candidates.length > 1
              ? 'Выбрать другой вариант'
              : tv
                ? 'Не тот сериал? Уточнить название →'
                : 'Не тот фильм? Уточнить название →'}
          </Button>
        </>
      ) : waiting ? (
        <div className="stack">
          <div className="search-progress">
            <span className="spinner spinner--large" />
            {reel ? (
              <>
                <h2>Читаем описание рилса</h2>
                <p className="muted search-progress__text">
                  Ищем название в кавычках, после «фильм:» или рядом с годом.
                  Пара секунд.
                </p>
                {flow.job?.source_url && <ReelLink url={flow.job.source_url} />}
              </>
            ) : (
              <>
                <h2>Ищем фильмы и сериалы</h2>
                <p className="muted">Обычно это занимает несколько секунд.</p>
              </>
            )}
          </div>
          <InlineError message={flow.error} />
          {flow.pollStopped && (
            <Button onClick={flow.resume}>Продолжить ожидание</Button>
          )}
          <Button
            variant="secondary"
            busy={flow.busy}
            onClick={() => void flow.reset()}
          >
            Отменить поиск
          </Button>
        </div>
      ) : reel ? (
        <ReelFailure
          code={flow.job?.error_code}
          title={flow.job?.query_title}
          onEnterTitle={() => void enterTitle(flow.job?.query_title ?? '')}
        />
      ) : (
        <EmptyState
          title="Ничего не найдено"
          description={
            flow.error || 'Попробуй уточнить название или год выхода.'
          }
          action="Изменить запрос"
          onAction={() => void flow.reset()}
        />
      )}
    </Shell>
  )
}

// Рилс не превратился в кандидатов. Причины разные, а выход один —
// ввести название вручную, поэтому кнопка всегда ведёт к форме.
function ReelFailure({
  code,
  title,
  onEnterTitle,
}: {
  code?: string
  title?: string
  onEnterTitle: () => void
}) {
  const notInCatalog = code === 'no_candidates' && !!title
  const notRecognized = code === 'title_not_recognized'
  return (
    <div className="stack reel-failure">
      <div className="reel-failure__head">
        <ReelIcon size="lg" />
        <h2>
          {notInCatalog
            ? `Не нашли «${title}» в кинобазе`
            : notRecognized
              ? 'Не нашли название в описании'
              : 'Не получилось открыть рилс'}
        </h2>
        <p className="muted">
          {notInCatalog
            ? 'Название взяли из описания рилса. Уточни его или добавь год.'
            : notRecognized
              ? 'Автор рилса не написал название явно. Введи его сам — займёт пару секунд.'
              : 'Instagram не отдал описание. Попробуй позже или введи название сам.'}
        </p>
      </div>
      {!notInCatalog && (
        <div className="reel-failure__tip">
          <p className="muted small">
            Узнаём автоматически, если в описании есть
          </p>
          <CaptionPatterns />
        </div>
      )}
      <Button onClick={onEnterTitle}>
        {notInCatalog ? 'Уточнить название' : 'Ввести название'}
      </Button>
    </div>
  )
}
