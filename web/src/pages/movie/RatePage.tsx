import type { UserMovie } from '../../api/types'
import { api } from '../../api/client'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { useResource } from '../../shared/hooks/useResource'
import { useMutation } from '../../shared/hooks/useMutation'
import { ErrorState, SkeletonList } from '../../shared/ui/Feedback'
import { RatingForm } from '../../features/rating/RatingForm'
import { movieMetadata } from '../../entities/movie/metadata'
function RateEditor({
  item,
  back,
  changed,
}: {
  item: UserMovie
  back: () => void
  changed: (message: string) => void
}) {
  const mutation = useMutation()
  return (
    <RatingForm
      title={`${item.movie.title} · ${movieMetadata(item.movie)}`}
      initialRating={item.user_rating}
      initialReview={item.review}
      busy={mutation.busy}
      error={mutation.error}
      onSave={(rating, review) =>
        void mutation.execute(
          () =>
            api.updateMovie(item.id, {
              status: 'watched',
              user_rating: rating,
              review,
            }),
          () => {
            changed('Просмотр сохранён')
            back()
          },
        )
      }
    />
  )
}
export function RatePage({
  id,
  navigate,
  back,
  changed,
}: {
  id: number
  navigate: Navigate
  back: () => void
  changed: (message: string) => void
}) {
  const resource = useResource((signal) => api.getMovie(id, signal), id)
  return (
    <Shell
      title={
        resource.data?.movie.media_type === 'tv'
          ? 'Как тебе сериал?'
          : resource.data
            ? 'Как тебе фильм?'
            : 'Оценка и отзыв'
      }
      navigate={navigate}
      onBack={back}
    >
      {resource.loading ? (
        <SkeletonList />
      ) : resource.error ? (
        <ErrorState message={resource.error} onRetry={resource.retry} />
      ) : (
        resource.data && (
          <RateEditor item={resource.data} back={back} changed={changed} />
        )
      )}
    </Shell>
  )
}
