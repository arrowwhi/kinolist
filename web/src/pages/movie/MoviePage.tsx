import { DirectorCredits } from '../../entities/director/DirectorCredits'
import { useState } from 'react'
import { api } from '../../api/client'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { MovieHero } from '../../entities/movie/MovieHero'
import { useResource } from '../../shared/hooks/useResource'
import { useMutation } from '../../shared/hooks/useMutation'
import { Button } from '../../shared/ui/Button'
import { ConfirmDialog } from '../../shared/ui/Dialog'
import {
  EmptyState,
  ErrorState,
  InlineError,
  SkeletonList,
} from '../../shared/ui/Feedback'
import { formatDate, safeExternalUrl } from '../../shared/lib/format'
export function MoviePage({
  id,
  revision,
  navigate,
  back,
  changed,
}: {
  id: number
  revision: number
  navigate: Navigate
  back: () => void
  changed: (message: string) => void
}) {
  const {
    data: item,
    error,
    loading,
    retry,
  } = useResource((signal) => api.getMovie(id, signal), `${id}:${revision}`)
  const mutation = useMutation()
  const [confirmDelete, setConfirmDelete] = useState(false)
  const source = safeExternalUrl(item?.source_url)
  const watched = item?.status === 'watched'
  return (
    <Shell
      title={item?.movie.title ?? 'Карточка'}
      navigate={navigate}
      onBack={() => {
        if (!mutation.busy) back()
      }}
    >
      {loading ? (
        <SkeletonList />
      ) : error ? (
        <ErrorState message={error} onRetry={retry} />
      ) : !item ? (
        <EmptyState title="Запись не найдена" />
      ) : (
        <>
          {item.movie.original_title &&
            item.movie.original_title !== item.movie.title && (
              <p className="original-title muted">
                {item.movie.original_title}
              </p>
            )}
          <MovieHero
            movie={item.movie}
            onGenre={(genre) => navigate(`/genres/${genre.id}`)}
          />
          <DirectorCredits
            key={id}
            directors={item.movie.directors}
            onOpen={(directorId) =>
              navigate(`/directors/${directorId}?status=all`)
            }
          />
          <section className="stack stack--tight">
            <h2>{item.movie.media_type === 'tv' ? 'О сериале' : 'О фильме'}</h2>
            <p className="muted overview">
              {item.movie.overview || 'В каталоге пока нет описания.'}
            </p>
          </section>
          <section className="stack stack--tight">
            <h2>Моя заметка</h2>
            <button
              type="button"
              className="note-preview"
              onClick={() => navigate(`/movie/${id}/note`)}
            >
              <span>{item.note || 'Например, кто посоветовал посмотреть'}</span>
              <small>Изменить заметку</small>
            </button>
          </section>
          <p className="muted small">
            Добавлено в коллекцию · {formatDate(item.added_at)}
          </p>
          {watched && (
            <section className="stack stack--tight">
              <div className="personal-rating">
                <strong className="accent">
                  {item.user_rating ? `${item.user_rating}/10` : 'Без оценки'}
                </strong>
                <span className="muted small">
                  Просмотрено {formatDate(item.watched_at)}
                </span>
              </div>
              <h2>Мой отзыв</h2>
              <p className={item.review ? 'review-text' : 'muted'}>
                {item.review ||
                  'Пока без отзыва. Можно добавить свои впечатления.'}
              </p>
            </section>
          )}
          <InlineError message={mutation.error} />
          <div className="stack stack--tight">
            <Button
              onClick={() => navigate(`/movie/${id}/rate`)}
              disabled={mutation.busy}
            >
              {watched ? 'Изменить оценку и отзыв' : '✓ Отметить просмотренным'}
            </Button>
            {watched && (
              <Button
                variant="secondary"
                busy={mutation.busy}
                onClick={() =>
                  void mutation.execute(
                    () => api.updateMovie(id, { status: 'want_to_watch' }),
                    () => changed('Возвращено в «На потом»'),
                  )
                }
              >
                Вернуть в «На потом»
              </Button>
            )}
            {source && (
              <a
                className="button button--secondary"
                href={source}
                target="_blank"
                rel="noreferrer"
              >
                Открыть источник ↗
              </a>
            )}
            <Button
              variant="quiet"
              disabled={mutation.busy}
              onClick={() => setConfirmDelete(true)}
            >
              Удалить из коллекции
            </Button>
          </div>
          {confirmDelete && (
            <ConfirmDialog
              title={
                item.movie.media_type === 'tv'
                  ? 'Удалить сериал?'
                  : 'Удалить фильм?'
              }
              description={`«${item.movie.title}», заметка, оценка и отзыв будут удалены из твоей коллекции.`}
              busy={mutation.busy}
              onCancel={() => setConfirmDelete(false)}
              onConfirm={() =>
                void mutation.execute(
                  () => api.deleteMovie(id),
                  () => {
                    changed('Удалено из коллекции')
                    navigate(
                      watched
                        ? '/collection/watched'
                        : '/collection/want_to_watch',
                      true,
                    )
                  },
                )
              }
            />
          )}
        </>
      )}
    </Shell>
  )
}
