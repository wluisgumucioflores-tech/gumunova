import type { ChangeEvent } from 'react'

export interface SelectOption {
  value: string
  label: string
}

interface SelectFieldProps {
  id: string
  label: string
  value: string
  options: SelectOption[]
  onChange: (value: string) => void
  /** First, empty option that asks the user to choose. */
  placeholder: string
  error?: string
  hint?: string
  /** API path of the field, used to find the first invalid input on screen. */
  path?: string
}

export function SelectField({ id, label, value, options, onChange, placeholder, error, hint, path }: SelectFieldProps) {
  const hintId = hint ? `${id}-hint` : undefined
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined

  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <select
        id={id}
        data-path={path}
        value={value}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        onChange={(event: ChangeEvent<HTMLSelectElement>) => onChange(event.target.value)}
      >
        <option value="">{placeholder}</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {hint && (
        <p id={hintId} className="field-hint">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className="field-error">
          {error}
        </p>
      )}
    </div>
  )
}
