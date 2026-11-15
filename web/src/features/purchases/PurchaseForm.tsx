import { useRef, useState, type FormEvent } from 'react'
import { ApiError, messageForApiError, messageForFieldError } from '../../api/client'
import type { PurchaseInput } from '../../api/purchases'
import type { Product } from '../../api/products'
import type { Supplier } from '../../api/suppliers'
import { TrashIcon } from '../../components/icons'
import { IconButton } from '../../components/IconButton'
import { SelectField, type SelectOption } from '../../components/SelectField'
import { TextField } from '../../components/TextField'
import { es } from '../../i18n/es'
import { ProductFields } from '../catalog/ProductFields'
import type { FieldKey } from '../catalog/formModel'
import { formatDecimalEs } from '../../lib/decimal'
import {
  NEW_PRODUCT,
  emptyPurchase,
  estimateTotals,
  newIdempotencyKey,
  newPvcLine,
  newRoll,
  newTableclothLine,
  resizeRolls,
  MAX_ROLLS,
  toPurchaseInput,
  validatePurchase,
  withProductChoice,
  type LineForm,
  type PurchaseFormValues,
  type PvcLine,
  type RollRow,
  type TableclothLine,
} from './formModel'

const t = es.purchases.form

interface PurchaseFormProps {
  suppliers: Pick<Supplier, 'id' | 'name'>[]
  pvcProducts: ProductOption[]
  tableclothProducts: ProductOption[]
  /** Values of the purchase being corrected; without them the form registers a new purchase. */
  initialValues?: PurchaseFormValues
  /** True when stock from the purchase was sold: only the general data can then be corrected. */
  itemsLocked?: boolean
  /** Saves the purchase. It receives the key that makes a retry of this attempt harmless. */
  onSubmit: (input: PurchaseInput, idempotencyKey: string) => Promise<void>
  onCancel: () => void
}

export type ProductOption = Pick<Product, 'id' | 'sku' | 'name'>

type Errors = Partial<Record<string, string>>

const productOptions = (products: ProductOption[]): SelectOption[] =>
  products.map((item) => ({ value: item.id, label: `${item.sku} — ${item.name}` }))

export function PurchaseForm({
  suppliers,
  pvcProducts,
  tableclothProducts,
  initialValues,
  itemsLocked = false,
  onSubmit,
  onCancel,
}: PurchaseFormProps) {
  const editing = initialValues !== undefined
  const formRef = useRef<HTMLFormElement>(null)
  const [values, setValues] = useState<PurchaseFormValues>(() => initialValues ?? emptyPurchase())
  const [errors, setErrors] = useState<Errors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  // One key per form: pressing Register twice, or retrying after a timeout, is the same purchase.
  const [idempotencyKey] = useState(newIdempotencyKey)

  const totals = estimateTotals(values)

  function clearError(path: string) {
    setErrors((current) => (current[path] ? { ...current, [path]: undefined } : current))
  }

  function setField<K extends keyof PurchaseFormValues>(key: K, value: PurchaseFormValues[K], path: string) {
    setValues((current) => ({ ...current, [key]: value }))
    clearError(path)
  }

  function updateLine(lineKey: string, change: (line: LineForm) => LineForm) {
    setValues((current) => ({
      ...current,
      lines: current.lines.map((line) => (line.key === lineKey ? change(line) : line)),
    }))
  }

  function showErrors(problems: Errors) {
    setErrors(problems)
    // Focus the first invalid field in screen order, not in the order the problems were found.
    const inputs = Array.from(formRef.current?.querySelectorAll<HTMLElement>('[data-path]') ?? [])
    inputs.find((input) => problems[input.dataset.path ?? ''])?.focus()
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) return

    const codes = validatePurchase(values)
    if (Object.keys(codes).length > 0) {
      setFormError(t.fixErrors)
      showErrors(Object.fromEntries(Object.entries(codes).map(([path, code]) => [path, messageForFieldError(code)])))
      return
    }

    setSubmitting(true)
    setFormError(null)
    setErrors({})
    try {
      await onSubmit(toPurchaseInput(values), idempotencyKey)
    } catch (error) {
      if (error instanceof ApiError && error.code === 'validation_failed') {
        setFormError(t.fixErrors)
        showErrors(Object.fromEntries(error.fields.map((item) => [item.field, messageForFieldError(item.code)])))
      } else {
        setFormError(`${editing ? t.saveError : t.submitError} ${messageForApiError(error)}`)
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="form" ref={formRef} onSubmit={handleSubmit} noValidate>
      <p className="notice">{editing ? (itemsLocked ? t.itemsLockedNotice : t.editNotice) : t.registerNotice}</p>

      {formError && (
        <p className="form-error" role="alert">
          {formError}
        </p>
      )}

      <fieldset className="form-section">
        <legend>{t.sections.general}</legend>
        <div className="form-grid">
          <SelectField
            id="purchase-supplier"
            path="supplier_id"
            label={t.fields.supplier}
            placeholder={t.supplierPlaceholder}
            value={values.supplierId}
            options={suppliers.map((item) => ({ value: item.id, label: item.name }))}
            error={errors.supplier_id}
            onChange={(value) => setField('supplierId', value, 'supplier_id')}
          />
          <TextField
            id="purchase-date"
            path="purchase_date"
            type="date"
            label={t.fields.date}
            value={values.purchaseDate}
            error={errors.purchase_date}
            onChange={(value) => setField('purchaseDate', value, 'purchase_date')}
          />
          <TextField
            id="purchase-reference"
            path="document_reference"
            label={t.fields.reference}
            optional
            maxLength={60}
            hint={t.fields.referenceHint}
            value={values.documentReference}
            error={errors.document_reference}
            onChange={(value) => setField('documentReference', value, 'document_reference')}
          />
        </div>
        <TextField
          id="purchase-notes"
          path="notes"
          label={t.fields.notes}
          optional
          multiline
          maxLength={500}
          value={values.notes}
          error={errors.notes}
          onChange={(value) => setField('notes', value, 'notes')}
        />
      </fieldset>

      <fieldset className="form-section" disabled={itemsLocked}>
        <legend>{t.sections.items}</legend>

        {values.lines.length === 0 && <p className="muted">{t.noItems}</p>}
        {errors.lines && (
          <p className="field-error" role="alert">
            {errors.lines}
          </p>
        )}

        {values.lines.map((line, index) =>
          line.kind === 'pvc' ? (
            <PvcLineCard
              key={line.key}
              line={line}
              index={index}
              products={productOptions(pvcProducts)}
              errors={errors}
              onChange={(change) => updateLine(line.key, change as (item: LineForm) => LineForm)}
              onRemove={() => setValues((current) => ({ ...current, lines: current.lines.filter((item) => item.key !== line.key) }))}
              clearError={clearError}
            />
          ) : (
            <TableclothLineCard
              key={line.key}
              line={line}
              index={index}
              products={productOptions(tableclothProducts)}
              errors={errors}
              onChange={(change) => updateLine(line.key, change as (item: LineForm) => LineForm)}
              onRemove={() => setValues((current) => ({ ...current, lines: current.lines.filter((item) => item.key !== line.key) }))}
              clearError={clearError}
            />
          ),
        )}

        <div className="form-actions">
          <button
            type="button"
            className="button"
            onClick={() => {
              setValues((current) => ({ ...current, lines: [...current.lines, newPvcLine()] }))
              clearError('lines')
            }}
          >
            {t.addPvc}
          </button>
          <button
            type="button"
            className="button"
            onClick={() => {
              setValues((current) => ({ ...current, lines: [...current.lines, newTableclothLine()] }))
              clearError('lines')
            }}
          >
            {t.addTablecloth}
          </button>
        </div>
      </fieldset>

      <fieldset className="form-section" disabled={itemsLocked}>
        <legend>{t.sections.costs}</legend>
        <TextField
          id="purchase-additional"
          path="additional_costs"
          label={t.fields.additionalCosts}
          optional
          inputMode="decimal"
          hint={t.fields.additionalCostsHint}
          value={values.additionalCosts}
          error={errors.additional_costs}
          onChange={(value) => setField('additionalCosts', value, 'additional_costs')}
        />
      </fieldset>

      <section className="summary" aria-label={t.sections.summary}>
        <h2>{t.sections.summary}</h2>
        <dl>
          <div>
            <dt>{t.summary.goods}</dt>
            <dd>{es.purchases.money(formatDecimalEs(totals.goods))}</dd>
          </div>
          <div>
            <dt>{t.summary.additional}</dt>
            <dd>{es.purchases.money(formatDecimalEs(totals.additional))}</dd>
          </div>
          <div className="summary-total">
            <dt>{t.summary.total}</dt>
            <dd>{es.purchases.money(formatDecimalEs(totals.total))}</dd>
          </div>
        </dl>
        <p className="field-hint">{t.summary.hint}</p>
      </section>

      <div className="form-actions">
        <button type="submit" className="button button-primary" disabled={submitting}>
          {submitting ? (editing ? t.saving : t.submitting) : editing ? t.save : t.submit}
        </button>
        <button type="button" className="button" onClick={onCancel} disabled={submitting}>
          {es.common.cancel}
        </button>
      </div>
    </form>
  )
}

interface LineCardProps<L> {
  line: L
  index: number
  products: SelectOption[]
  errors: Errors
  onChange: (change: (line: L) => L) => void
  onRemove: () => void
  clearError: (path: string) => void
}

function PvcLineCard({ line, index, products, errors, onChange, onRemove, clearError }: LineCardProps<PvcLine>) {
  const prefix = `lines[${index}]`
  const number = index + 1
  // What is being typed in "Cantidad de rollos"; null shows the real number of rows.
  const [rollCountDraft, setRollCountDraft] = useState<string | null>(null)

  function updateRoll(rollKey: string, field: 'length' | 'cost', value: string, rollIndex: number) {
    onChange((current) => ({
      ...current,
      rolls: current.rolls.map((roll) => (roll.key === rollKey ? { ...roll, [field]: value } : roll)),
    }))
    clearError(`${prefix}.rolls[${rollIndex}].${field === 'length' ? 'length_m' : 'cost'}`)
  }

  return (
    <section className="line-card" aria-label={t.pvcLineTitle(number)}>
      <header className="line-card-header">
        <h3>{t.pvcLineTitle(number)}</h3>
        <IconButton label={t.removeLine(number)} icon={<TrashIcon />} variant="danger" onClick={onRemove} />
      </header>

      <ProductChoice kind={'pvc'} line={line} prefix={prefix} products={products} errors={errors} onChange={onChange} clearError={clearError} />

      <TextField
        id={`line-${line.key}-roll-count`}
        label={t.rollCount}
        inputMode="decimal"
        hint={t.rollCountHint}
        value={rollCountDraft ?? String(line.rolls.length)}
        onChange={(value) => {
          setRollCountDraft(value)
          // Only a whole number from 1 up resizes the rows; anything else waits until it is fixed.
          if (/^\d{1,3}$/.test(value.trim()) && Number(value) >= 1 && Number(value) <= MAX_ROLLS) {
            onChange((current) => ({ ...current, rolls: resizeRolls(current.rolls, Number(value)) }))
            clearError(`${prefix}.rolls`)
          }
        }}
        onBlur={() => setRollCountDraft(null)}
      />

      <h4>{t.rollsHeading}</h4>
      {errors[`${prefix}.rolls`] && (
        <p className="field-error" role="alert">
          {errors[`${prefix}.rolls`]}
        </p>
      )}
      <ul className="roll-list">
        {line.rolls.map((roll: RollRow, rollIndex) => {
          const rollPrefix = `${prefix}.rolls[${rollIndex}]`
          return (
            <li key={roll.key} className="roll-row">
              <span className="roll-number">{t.rollTitle(rollIndex + 1)}</span>
              <TextField
                id={`roll-${roll.key}-length`}
                path={`${rollPrefix}.length_m`}
                label={t.rollLength}
                inputMode="decimal"
                value={roll.length}
                error={errors[`${rollPrefix}.length_m`]}
                onChange={(value) => updateRoll(roll.key, 'length', value, rollIndex)}
              />
              <TextField
                id={`roll-${roll.key}-cost`}
                path={`${rollPrefix}.cost`}
                label={t.rollCost}
                inputMode="decimal"
                value={roll.cost}
                error={errors[`${rollPrefix}.cost`]}
                onChange={(value) => updateRoll(roll.key, 'cost', value, rollIndex)}
              />
              <IconButton
                label={t.removeRoll(number, rollIndex + 1)}
                icon={<TrashIcon />}
                variant="danger"
                disabled={line.rolls.length === 1}
                onClick={() => onChange((current) => ({ ...current, rolls: current.rolls.filter((item) => item.key !== roll.key) }))}
              />
            </li>
          )
        })}
      </ul>
      <div className="form-actions">
        <button
          type="button"
          className="button"
          onClick={() =>
            onChange((current) => ({ ...current, rolls: [...current.rolls, newRoll(current.rolls[current.rolls.length - 1])] }))
          }
        >
          {t.addRoll}
        </button>
      </div>
    </section>
  )
}

function TableclothLineCard({ line, index, products, errors, onChange, onRemove, clearError }: LineCardProps<TableclothLine>) {
  const prefix = `lines[${index}]`
  const number = index + 1

  return (
    <section className="line-card" aria-label={t.tableclothLineTitle(number)}>
      <header className="line-card-header">
        <h3>{t.tableclothLineTitle(number)}</h3>
        <IconButton label={t.removeLine(number)} icon={<TrashIcon />} variant="danger" onClick={onRemove} />
      </header>

      <ProductChoice kind={'tablecloth'} line={line} prefix={prefix} products={products} errors={errors} onChange={onChange} clearError={clearError} />
      <div className="form-grid">
        <TextField
          id={`line-${line.key}-quantity`}
          path={`${prefix}.quantity`}
          label={t.quantity}
          inputMode="decimal"
          value={line.quantity}
          error={errors[`${prefix}.quantity`]}
          onChange={(value) => {
            onChange((current) => ({ ...current, quantity: value }))
            clearError(`${prefix}.quantity`)
          }}
        />
        <TextField
          id={`line-${line.key}-unit-cost`}
          path={`${prefix}.unit_cost`}
          label={t.unitCost}
          inputMode="decimal"
          value={line.unitCost}
          error={errors[`${prefix}.unit_cost`]}
          onChange={(value) => {
            onChange((current) => ({ ...current, unitCost: value }))
            clearError(`${prefix}.unit_cost`)
          }}
        />
      </div>
    </section>
  )
}

interface ProductChoiceProps<L extends LineForm> {
  kind: L['kind']
  line: L
  prefix: string
  products: SelectOption[]
  errors: Errors
  onChange: (change: (line: L) => L) => void
  clearError: (path: string) => void
}

/** Picks an existing product, or asks for the fields of a new one that the purchase will create. */
function ProductChoice<L extends LineForm>({ kind, line, prefix, products, errors, onChange, clearError }: ProductChoiceProps<L>) {
  const newProductPrefix = `${prefix}.new_product.`

  function setProductField(key: FieldKey, value: string) {
    onChange((current) => (current.newProduct ? { ...current, newProduct: { ...current.newProduct, [key]: value } } : current))
    clearError(`${newProductPrefix}${key}`)
  }

  // Errors of the new product are stored under their full API path; ProductFields reads them by field key.
  const productErrors = Object.fromEntries(
    Object.entries(errors)
      .filter(([path]) => path.startsWith(newProductPrefix))
      .map(([path, message]) => [path.slice(newProductPrefix.length), message]),
  )

  return (
    <>
      <SelectField
        id={`line-${line.key}-product`}
        path={`${prefix}.product_id`}
        label={t.productLabel}
        placeholder={t.productPlaceholder}
        value={line.newProduct ? NEW_PRODUCT : line.productId}
        options={[{ value: NEW_PRODUCT, label: t.newProductOption }, ...products]}
        error={errors[`${prefix}.product_id`]}
        onChange={(value) => {
          onChange((current) => withProductChoice(current, value))
          clearError(`${prefix}.product_id`)
        }}
      />
      {line.newProduct && (
        <div className="new-product">
          <p className="field-hint">{t.newProductHint}</p>
          <ProductFields
            type={kind}
            values={line.newProduct}
            errors={productErrors}
            onChange={setProductField}
            idPrefix={`line-${line.key}-new-`}
            pathPrefix={newProductPrefix}
          />
        </div>
      )}
    </>
  )
}
