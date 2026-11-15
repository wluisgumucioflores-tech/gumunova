import { useState, type FormEvent, type ReactNode } from 'react'
import { ApiError, messageForApiError, messageForFieldError } from '../../api/client'
import type { Product, ProductInput, ProductType } from '../../api/products'
import { CheckCircleIcon, CircleIcon, ImageIcon, InfoIcon, SaveIcon } from '../../components/icons'
import { ProductFields } from './ProductFields'
import { es } from '../../i18n/es'
import {
  emptyValues,
  fieldsFor,
  toProductInput,
  validate,
  valuesFromProduct,
  type FieldKey,
  type FormErrors,
  type FormValues,
} from './formModel'

interface ProductFormProps {
  /** Present when editing: SKU and type are then read-only. */
  product?: Product
  onSubmit: (input: ProductInput) => Promise<void>
  onCancel: () => void
  /** Optional extra section shown before the buttons, for example the image picker. */
  imageSlot?: ReactNode
  /** When given, the form shows its own page header: this title, the type badge and a subtitle. */
  title?: string
}

const fieldId = (key: FieldKey) => `field-${key.replace('.', '-')}`

export function ProductForm({ product, onSubmit, onCancel, imageSlot, title }: ProductFormProps) {
  const isEditing = product !== undefined
  const [type, setType] = useState<ProductType>(product?.product_type ?? 'pvc')
  const [values, setValues] = useState<FormValues>(() => (product ? valuesFromProduct(product) : emptyValues()))
  const [errors, setErrors] = useState<FormErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function setValue(key: FieldKey, value: string) {
    setValues((current) => ({ ...current, [key]: value }))
    if (errors[key]) {
      setErrors((current) => ({ ...current, [key]: undefined }))
    }
  }

  function showErrors(problems: FormErrors) {
    setErrors(problems)
    const firstInvalid = fieldsFor(type).find((key) => problems[key])
    if (firstInvalid) {
      document.getElementById(fieldId(firstInvalid))?.focus()
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) return

    const codes = validate(type, values)
    if (Object.keys(codes).length > 0) {
      setFormError(es.form.fixErrors)
      showErrors(Object.fromEntries(Object.entries(codes).map(([key, code]) => [key, messageForFieldError(code)])))
      return
    }

    setSubmitting(true)
    setFormError(null)
    setErrors({})
    try {
      await onSubmit(toProductInput(type, values))
    } catch (error) {
      handleSubmitError(error)
    } finally {
      setSubmitting(false)
    }
  }

  function handleSubmitError(error: unknown) {
    if (error instanceof ApiError && error.code === 'validation_failed') {
      setFormError(es.form.fixErrors)
      showErrors(Object.fromEntries(error.fields.map((field) => [field.field, messageForFieldError(field.code)])))
      return
    }
    if (error instanceof ApiError && error.code === 'sku_already_exists') {
      setFormError(es.form.fixErrors)
      showErrors({ sku: messageForApiError(error) })
      return
    }
    setFormError(`${es.form.submitError} ${messageForApiError(error)}`)
  }

  const typeSelector = isEditing ? (
    <div className="field">
      <span className="field-label">{es.form.fields.productType}</span>
      <p className="field-static">{es.catalog.types[type]}</p>
      <p className="field-hint">{es.form.productTypeLocked}</p>
    </div>
  ) : (
    <div className="field" role="radiogroup" aria-labelledby="product-type-label">
      <span id="product-type-label" className="field-label">
        {es.form.fields.productType}
      </span>
      <div className="segmented">
        {(['pvc', 'tablecloth'] as const).map((option) => (
          <label key={option} className={type === option ? 'segmented-option is-selected' : 'segmented-option'}>
            <input
              type="radio"
              name="product_type"
              value={option}
              checked={type === option}
              onChange={() => {
                setType(option)
                setErrors({})
                setFormError(null)
              }}
            />
            {type === option ? <CheckCircleIcon /> : <CircleIcon />}
            {es.catalog.types[option]}
          </label>
        ))}
      </div>
    </div>
  )

  return (
    <form className="form" onSubmit={handleSubmit} noValidate>
      {title && (
        <header className="page-header">
          <div>
            <div className="page-title">
              <h1>{title}</h1>
              <span className="badge-type">{es.catalog.types[type]}</span>
            </div>
            <p className="page-subtitle">{es.form.createSubtitle[type]}</p>
          </div>
        </header>
      )}

      {formError && (
        <p className="form-error" role="alert">
          {formError}
        </p>
      )}

      <ProductFields
        type={type}
        isEditing={isEditing}
        values={values}
        errors={errors}
        onChange={setValue}
        lead={typeSelector}
      />

      {imageSlot && (
        <fieldset className="form-section">
          <legend>
            <ImageIcon />
            {es.form.sections.image}
          </legend>
          {imageSlot}
        </fieldset>
      )}

      <div className="form-footer">
        <p className="form-footer-note">
          <InfoIcon />
          {es.form.requiredNote}
        </p>
        <div className="form-actions">
          <button type="button" className="button" onClick={onCancel} disabled={submitting}>
            {es.common.cancel}
          </button>
          <button type="submit" className="button button-primary" disabled={submitting}>
            {!submitting && <SaveIcon />}
            {submitting ? es.common.saving : es.form.submit}
          </button>
        </div>
      </div>
    </form>
  )
}
