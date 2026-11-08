import { useEffect, useState } from 'react'
import { RequestError } from '../../api/client'
import { catalogApi } from '../../api/catalog'
import type { AddFromCatalog } from '../../api/catalog'
import type { UserMovie } from '../../api/types'
import type { CatalogRoute, Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { MoviePreview } from '../../entities/movie/MoviePreview'
import { movieMetadata, upcomingLabel } from '../../entities/movie/metadata'
import { RatingForm } from '../../features/rating/RatingForm'
import { useResource } from '../../shared/hooks/useResource'
import { useMutation } from '../../shared/hooks/useMutation'
import { Button } from '../../shared/ui/Button'
import { ErrorState, InlineError, SkeletonList } from '../../shared/ui/Feedback'

// Карточка тайтла из каталога, которого ещё нет в коллекции (экраны
// «Каталог / не в коллекции» и «Каталог / скоро»): выглядит как экран
// подтверждения при добавлении — MoviePreview и те же кнопки. Открывается из
// фильмографии режиссёра; после добавления заменяется карточкой коллекции,
// так что «назад» возвращает к профилю режиссёра.
export function CatalogMoviePage({
  route,
  navigate,
  back,
  changed,
}: {
  route: CatalogRoute
  navigate: Navigate
  back: () => void
  changed: (message: string) => void
}) {
  const { provider, mediaType, externalId, directorId } = route
  const resource = useResource(
    (signal) =>
      catalogApi.get(provider, mediaType, externalId, signal).catch((error) => {
        if (error instanceof RequestError && error.status === 404)
          throw new RequestError(error.code, 'Не нашли в каталоге')
        throw error
      }),
    `${provider}:${mediaType}:${externalId}`,
  )
  const movie = resource.data
  const mutation = useMutation()
  const [rating, setRating] = useState(false)
  const ownedId = movie?.user_movie_id
  // Тайтл уже в коллекции — показываем его настоящую карточку.
  useEffect(() => {
    if (ownedId) navigate(`/movie/${ownedId}`, true)
  }, [ownedId, navigate])

  const save = (
    options: Pick<AddFromCatalog, 'status' | 'user_rating' | 'review'>,
  ) =>
    void mutation.execute(
      async (): Promise<{ added: UserMovie } | { existing: number }> => {
        try {
          return {
            added: await catalogApi.add({
              provider,
              external_id: externalId,
              media_type: mediaType,
              ...options,
            }),
          }
        } catch (error) {
          if (error instanceof RequestError && error.existingUserMovieId)
            return { existing: error.existingUserMovieId }
          throw error
        }
      },
      (result) => {
        if ('existing' in result) {
          changed('Уже в коллекции')
          navigate(`/movie/${result.existing}`, true)
          return
        }
        changed(
          result.added.status === 'watched'
            ? 'Просмотр сохранён'
            : `«${movie?.title ?? result.added.movie.title}» — в «На потом»`,
        )
        navigate(`/movie/${result.added.id}`, true)
      },
    )

  const tv = mediaType === 'tv'
  const director = directorId
    ? movie?.directors?.find((d) => d.id === directorId)
    : undefined
  return (
    <Shell
      title={
        rating
          ? tv
            ? 'Как тебе сериал?'
            : 'Как тебе фильм?'
          : tv
            ? 'Сериал'
            : 'Фильм'
      }
      navigate={navigate}
      onBack={() => {
        if (mutation.busy) return
        if (rating) setRating(false)
        else back()
      }}
    >
      {resource.loading || ownedId ? (
        <SkeletonList />
      ) : resource.error || !movie ? (
        <ErrorState
          message={resource.error || 'Не нашли в каталоге'}
          onRetry={resource.retry}
        />
      ) : rating ? (
        <RatingForm
          title={`${movie.title} · ${movieMetadata(movie)}`}
          busy={mutation.busy}
          error={mutation.error}
          onSave={(value, review) =>
            save({
              status: 'watched',
              user_rating: value || undefined,
              review,
            })
          }
        />
      ) : (
        <>
          <MoviePreview
            movie={movie}
            upcoming={
              movie.upcoming
                ? upcomingLabel(movie.release_date, movie.year)
                : undefined
            }
            onDirector={(id) => navigate(`/v2/directors/${id}`)}
          />
          <p className="muted small">
            {[
              'Нет в коллекции',
              movie.upcoming
                ? 'ещё не вышел'
                : director
                  ? `из фильмографии: ${director.name}`
                  : '',
            ]
              .filter(Boolean)
              .join(' · ')}
          </p>
          <InlineError message={mutation.error} />
          <div className="stack">
            <Button
              busy={mutation.busy}
              onClick={() => save({ status: 'want_to_watch' })}
            >
              ＋ Сохранить на потом
            </Button>
            {!movie.upcoming && (
              <Button
                variant="secondary"
                disabled={mutation.busy}
                onClick={() => setRating(true)}
              >
                Уже смотрел
              </Button>
            )}
          </div>
        </>
      )}
    </Shell>
  )
}
