import { useId } from 'react'
import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react'

type InputProps = InputHTMLAttributes<HTMLInputElement> & { label?: string }
export function Field({ label, className = '', ...props }: InputProps) {
  const id = useId()
  return (
    <label className={`field-label ${className}`} htmlFor={props.id ?? id}>
      {label && <span>{label}</span>}
      <input {...props} id={props.id ?? id} className="field" />
    </label>
  )
}
type TextProps = TextareaHTMLAttributes<HTMLTextAreaElement> & {
  label: string
  hint?: string
}
export function TextArea({ label, hint, ...props }: TextProps) {
  const id = useId()
  return (
    <label className="field-label" htmlFor={id}>
      <span>{label}</span>
      <textarea {...props} id={id} className="field field--multiline" />
      {hint && <small className="muted">{hint}</small>}
    </label>
  )
}
