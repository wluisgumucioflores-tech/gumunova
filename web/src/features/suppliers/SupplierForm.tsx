import { useState, type FormEvent } from 'react'
import { ApiError, messageForApiError, messageForFieldError } from '../../api/client'
import type { Supplier, SupplierInput } from '../../api/suppliers'
import { TextField } from '../../components/TextField'
import { es } from '../../i18n/es'
import {
  SUPPLIER_FIELDS,
  emptySupplierValues,
  toSupplierInput,
  validateSupplier,
  valuesFromSupplier,
  type SupplierField,
  type SupplierValues,
} from './formModel'

interface SupplierFormProps {
  supplier?: Supplier
  onSubmit: (input: SupplierInput) => Promise<void>
  onCancel: () => void
}

type Errors = Partial<Record<string, string>>

const fieldId = (field: SupplierField) => `supplier-${field}`
const t = es.suppliers

export function SupplierForm({ supplier, onSubmit, onCancel }: SupplierFormProps) {
  const [values, setValues] = useState<SupplierValues>(() =>
    supplier ? valuesFromSupplier(supplier) : emptySupplierValues(),
  )
  const [errors, setErrors] = useState<Errors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function setValue(field: SupplierField, value: string) {
    setValues((current) => ({ ...current, [field]: value }))
    if (errors[field]) setErrors((current) => ({ ...current, [field]: undefined }))
  }

  function showErrors(problems: Errors) {
    setErrors(problems)
    const firstInvalid = SUPPLIER_FIELDS.find((field) => problems[field])
    if (firstInvalid) document.getElementById(fieldId(firstInvalid))?.focus()
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) return

    const codes = validateSupplier(values)
    if (Object.keys(codes).length > 0) {
      setFormError(t.fixErrors)
      showErrors(Object.fromEntries(Object.entries(codes).map(([field, code]) => [field, messageForFieldError(code)])))
      return
    }

    setSubmitting(true)
    setFormError(null)
    setErrors({})
    try {
      await onSubmit(toSupplierInput(values))
    } catch (error) {
      if (error instanceof ApiError && error.code === 'validation_failed') {
        setFormError(t.fixErrors)
        showErrors(Object.fromEntries(error.fields.map((item) => [item.field, messageForFieldError(item.code)])))
      } else {
        setFormError(`${t.submitError} ${messageForApiError(error)}`)
      }
    } finally {
      setSubmitting(false)
    }
  }

  const field = (key: SupplierField, label: string, options: Partial<Parameters<typeof TextField>[0]> = {}) => (
    <TextField
      id={fieldId(key)}
      label={label}
      value={values[key]}
      error={errors[key]}
      onChange={(value) => setValue(key, value)}
      {...options}
    />
  )

  return (
    <form className="form" onSubmit={handleSubmit} noValidate>
      {formError && (
        <p className="form-error" role="alert">
          {formError}
        </p>
      )}

      <fieldset className="form-section">
        <legend>{t.generalSection}</legend>
        {field('name', t.fields.name, { maxLength: 120 })}
        <div className="form-grid">
          {field('phone', t.fields.phone, { optional: true, maxLength: 30, type: 'tel' })}
          {field('email', t.fields.email, { optional: true, maxLength: 254, type: 'email' })}
        </div>
        {field('notes', t.fields.notes, { optional: true, multiline: true, maxLength: 500, hint: t.fields.notesHint })}
      </fieldset>

      <div className="form-actions">
        <button type="submit" className="button button-primary" disabled={submitting}>
          {submitting ? es.common.saving : es.common.save}
        </button>
        <button type="button" className="button" onClick={onCancel} disabled={submitting}>
          {es.common.cancel}
        </button>
      </div>
    </form>
  )
}
