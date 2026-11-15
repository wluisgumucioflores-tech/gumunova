import type { ChangeEvent, HTMLInputTypeAttribute, ReactNode } from 'react'
import { es } from '../i18n/es'

interface TextFieldProps {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  hint?: string
  /** Decorative icon shown before the hint. */
  hintIcon?: ReactNode
  error?: string
  optional?: boolean
  /** Marks the field as required with an asterisk. The backend still decides what is valid. */
  required?: boolean
  placeholder?: string
  /** Fixed text before the value, for example the currency. */
  prefix?: string
  /** Fixed text after the value, for example the unit. */
  suffix?: string
  readOnly?: boolean
  maxLength?: number
  inputMode?: 'text' | 'decimal'
  type?: HTMLInputTypeAttribute
  /** Renders a text area for longer notes. */
  multiline?: boolean
  /** API path of the field, used to find the first invalid input on screen. */
  path?: string
  autoComplete?: string
  onBlur?: () => void
}

export function TextField({
  id,
  label,
  value,
  onChange,
  hint,
  hintIcon,
  error,
  optional = false,
  required = false,
  placeholder,
  prefix,
  suffix,
  readOnly = false,
  maxLength,
  inputMode = 'text',
  type = 'text',
  multiline = false,
  path,
  autoComplete = 'off',
  onBlur,
}: TextFieldProps) {
  const hintId = hint ? `${id}-hint` : undefined
  const errorId = error ? `${id}-error` : undefined
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined

  return (
    <div className="field">
      <label htmlFor={id} className={required ? 'is-required' : undefined}>
        {label}
        {optional && <span className="field-optional">{es.common.optional}</span>}
      </label>
      {multiline ? (
        <textarea
          id={id}
          data-path={path}
          rows={4}
          placeholder={placeholder}
          aria-required={required || undefined}
          value={value}
          readOnly={readOnly}
          maxLength={maxLength}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          onChange={(event: ChangeEvent<HTMLTextAreaElement>) => onChange(event.target.value)}
        />
      ) : (
        <div className="input-wrap">
          {prefix && (
            <span className="input-prefix" aria-hidden="true">
              {prefix}
            </span>
          )}
          <input
            id={id}
            data-path={path}
            type={type}
            value={value}
            placeholder={placeholder}
            readOnly={readOnly}
            maxLength={maxLength}
            inputMode={inputMode}
            autoComplete={autoComplete}
            aria-required={required || undefined}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy}
            data-affix={prefix ? 'prefix' : undefined}
            onChange={(event: ChangeEvent<HTMLInputElement>) => onChange(event.target.value)}
            onBlur={onBlur}
          />
          {suffix && (
            <span className="input-suffix" aria-hidden="true">
              {suffix}
            </span>
          )}
        </div>
      )}
      {hint && (
        <p id={hintId} className="field-hint">
          {hintIcon}
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
