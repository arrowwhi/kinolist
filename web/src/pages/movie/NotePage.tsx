import { useState } from 'react'
import type { UserMovie } from '../../api/types'
import { api } from '../../api/client'
import type { Navigate } from '../../shared/lib/navigation'
import { Shell } from '../../widgets/app-shell/Shell'
import { useResource } from '../../shared/hooks/useResource'
import { useMutation } from '../../shared/hooks/useMutation'
import { Button } from '../../shared/ui/Button'
import { TextArea } from '../../shared/ui/Field'
import { ErrorState, InlineError, SkeletonList } from '../../shared/ui/Feedback'
function NoteForm({
  item,
  back,
  changed,
}: {
  item: UserMovie
  back: () => void
  changed: (message: string) => void
}) {
  const [note, setNote] = useState(item.note ?? '')
  const mutation = useMutation()
  return (
    <form
      className="stack"
      onSubmit={(e) => {
        e.preventDefault()
        void mutation.execute(
          () => api.updateMovie(item.id, { note }),
          () => {
            changed('Заметка сохранена')
            back()
          },
        )
      }}
    >
      <p className="muted">{item.movie.title}</p>
      <TextArea
        label="Моя заметка"
        rows={6}
        value={note}
        onChange={(e) => setNote(e.target.value)}
        placeholder="Кто посоветовал? С кем хочется посмотреть?"
        disabled={mutation.busy}
      />
      <p className="muted small">Личная заметка — только для тебя.</p>
      <InlineError message={mutation.error} />
      <div className="form-actions">
        <Button type="submit" busy={mutation.busy}>
          Сохранить заметку
        </Button>
        <Button variant="secondary" disabled={mutation.busy} onClick={back}>
          Отмена
        </Button>
      </div>
    </form>
  )
}
export function NotePage({
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
    <Shell title="Редактировать заметку" navigate={navigate} onBack={back}>
      {resource.loading ? (
        <SkeletonList />
      ) : resource.error ? (
        <ErrorState message={resource.error} onRetry={resource.retry} />
      ) : (
        resource.data && (
          <NoteForm item={resource.data} back={back} changed={changed} />
        )
      )}
    </Shell>
  )
}
