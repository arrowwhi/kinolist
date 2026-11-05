import { api } from '../../api/client'
import type { Genre } from '../../api/types'
import { useResource } from '../../shared/hooks/useResource'
import { ErrorState, SkeletonList } from '../../shared/ui/Feedback'
import { Shell } from '../../widgets/app-shell/Shell'
import { CollectionPage } from '../collection/CollectionPage'
import type { CollectionPreferences } from '../collection/CollectionPage'
import type { CollectionStatus } from '../../features/collection/model'
import type { Navigate } from '../../shared/lib/navigation'
export function DirectorWorksPage({
  id,
  ...props
}: {
  id: number
  status: CollectionStatus
  genres: Genre[]
  preferences: CollectionPreferences
  onPreferences: (value: CollectionPreferences) => void
  revision: number
  navigate: Navigate
  back: () => void
}) {
  const resource = useResource(
    (signal) => api.getDirector(id, signal),
    `${id}:${props.revision}`,
  )
  if (resource.loading || resource.error || !resource.data)
    return (
      <Shell
        title="Работы режиссёра"
        navigate={props.navigate}
        onBack={props.back}
      >
        {resource.loading ? (
          <SkeletonList />
        ) : (
          <ErrorState
            message={resource.error || 'Режиссёр не найден'}
            onRetry={resource.retry}
          />
        )}
      </Shell>
    )
  return <CollectionPage {...props} director={resource.data} />
}
