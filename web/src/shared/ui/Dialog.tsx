import { useEffect, useRef } from 'react'
import { Button } from './Button'
export function ConfirmDialog({
  title,
  description,
  busy,
  onConfirm,
  onCancel,
}: {
  title: string
  description: string
  busy: boolean
  onConfirm: () => void
  onCancel: () => void
}) {
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    const dialog = ref.current
    dialog?.showModal()
    return () => dialog?.close()
  }, [])
  return (
    <dialog
      ref={ref}
      className="dialog"
      aria-labelledby="dialog-title"
      onCancel={(e) => {
        e.preventDefault()
        if (!busy) onCancel()
      }}
    >
      <h2 id="dialog-title">{title}</h2>
      <p className="muted">{description}</p>
      <div className="stack">
        <Button variant="danger" busy={busy} onClick={onConfirm}>
          Удалить фильм
        </Button>
        <Button variant="secondary" disabled={busy} onClick={onCancel}>
          Отмена
        </Button>
      </div>
    </dialog>
  )
}
