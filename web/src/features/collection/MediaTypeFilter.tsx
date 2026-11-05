import type { MediaType } from '../../api/types'
export function MediaTypeFilter({
  value,
  onChange,
}: {
  value?: MediaType
  onChange: (value: MediaType | undefined) => void
}) {
  return (
    <fieldset>
      <legend>Тип</legend>
      <div className="media-type-options">
        {(
          [
            { value: undefined, label: 'Все' },
            { value: 'movie', label: 'Фильмы' },
            { value: 'tv', label: 'Сериалы' },
          ] as const
        ).map((option) => (
          <label
            key={option.label}
            className={`media-type-option ${value === option.value ? 'media-type-option--active' : ''}`}
          >
            <input
              type="radio"
              name="media_type"
              value={option.value ?? 'all'}
              checked={value === option.value}
              onChange={() => onChange(option.value)}
            />
            <span>{option.label}</span>
          </label>
        ))}
      </div>
    </fieldset>
  )
}
