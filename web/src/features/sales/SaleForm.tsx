import { useMemo, useRef, useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { ApiError, messageForApiError, messageForFieldError } from '../../api/client'
import type { Roll, StockItem } from '../../api/inventory'
import type { Product } from '../../api/products'
import type { SaleInput } from '../../api/sales'
import { TrashIcon } from '../../components/icons'
import { IconButton } from '../../components/IconButton'
import { SelectField } from '../../components/SelectField'
import { TextField } from '../../components/TextField'
import { es } from '../../i18n/es'
import { formatDecimalEs, formatUnitsEs, toInputDecimal } from '../../lib/decimal'
import { hundredthsToApi } from '../../lib/money'
import {
  emptySale,
  estimateLineTotal,
  estimateTotal,
  newFullRollLine,
  newIdempotencyKey,
  newRollMetersLine,
  newTableclothLine,
  toSaleInput,
  validateSale,
  type FullRollLine,
  type RollMetersLine,
  type SaleFormValues,
  type SaleLineForm,
  type TableclothLine,
} from './formModel'

const t = es.sales.form

interface SaleFormProps {
  /** Rolls with meters left. */
  rolls: Roll[]
  /** Stock of the tablecloth variants. */
  tablecloths: StockItem[]
  /** Active products, for the reference prices suggested to the seller. */
  products: Product[]
  /** Registers the sale. It receives the key that makes a retry of this attempt harmless. */
  onSubmit: (input: SaleInput, idempotencyKey: string) => Promise<void>
  onCancel: () => void
}

type Errors = Partial<Record<string, string>>

export function SaleForm({ rolls, tablecloths, products, onSubmit, onCancel }: SaleFormProps) {
  const formRef = useRef<HTMLFormElement>(null)
  const [values, setValues] = useState<SaleFormValues>(() => emptySale())
  const [errors, setErrors] = useState<Errors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  // One key per form: pressing Register twice, or retrying after a timeout, is the same sale.
  const [idempotencyKey] = useState(newIdempotencyKey)

  const productById = useMemo(() => new Map(products.map((product) => [product.id, product])), [products])
  const rollById = useMemo(() => new Map(rolls.map((roll) => [roll.id, roll])), [rolls])
  const clothById = useMemo(() => new Map(tablecloths.map((item) => [item.product_id, item])), [tablecloths])

  function clearError(path: string) {
    setErrors((current) => (current[path] ? { ...current, [path]: undefined } : current))
  }

  function updateLine(lineKey: string, change: (line: SaleLineForm) => SaleLineForm) {
    setValues((current) => ({ ...current, lines: current.lines.map((line) => (line.key === lineKey ? change(line) : line)) }))
  }

  function addLine(line: SaleLineForm) {
    setValues((current) => ({ ...current, lines: [...current.lines, line] }))
    clearError('lines')
  }

  function removeLine(lineKey: string) {
    setValues((current) => ({ ...current, lines: current.lines.filter((line) => line.key !== lineKey) }))
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

    const codes = validateSale(values)
    if (Object.keys(codes).length > 0) {
      setFormError(t.fixErrors)
      showErrors(Object.fromEntries(Object.entries(codes).map(([path, code]) => [path, messageForFieldError(code)])))
      return
    }

    setSubmitting(true)
    setFormError(null)
    setErrors({})
    try {
      await onSubmit(toSaleInput(values), idempotencyKey)
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

  const total = estimateTotal(values)
  const intactRolls = rolls.filter((roll) => roll.is_intact)
  const sellableCloths = tablecloths.filter((item) => Number(item.quantity.split('.')[0]) > 0)

  return (
    <form className="form" ref={formRef} onSubmit={handleSubmit} noValidate>
      <p className="notice">{t.immutableNotice}</p>

      {formError && (
        <p className="form-error" role="alert">
          {formError}
        </p>
      )}

      <fieldset className="form-section">
        <legend>{t.sections.general}</legend>
        <div className="form-grid">
          <TextField
            id="sale-date"
            path="sale_date"
            type="date"
            label={t.fields.date}
            value={values.saleDate}
            error={errors.sale_date}
            onChange={(value) => {
              setValues((current) => ({ ...current, saleDate: value }))
              clearError('sale_date')
            }}
          />
        </div>
        <TextField
          id="sale-notes"
          path="notes"
          label={t.fields.notes}
          optional
          multiline
          maxLength={500}
          value={values.notes}
          error={errors.notes}
          onChange={(value) => {
            setValues((current) => ({ ...current, notes: value }))
            clearError('notes')
          }}
        />
      </fieldset>

      <fieldset className="form-section">
        <legend>{t.sections.items}</legend>

        {values.lines.length === 0 && <p className="muted">{t.noItems}</p>}
        {errors.lines && (
          <p className="field-error" role="alert">
            {errors.lines}
          </p>
        )}

        {values.lines.map((line, index) => {
          const common = {
            index,
            errors,
            clearError,
            onRemove: () => removeLine(line.key),
          }
          if (line.kind === 'roll_meters') {
            return (
              <RollMetersCard
                key={line.key}
                line={line}
                rolls={rolls}
                rollById={rollById}
                productById={productById}
                onChange={(change) => updateLine(line.key, change as (item: SaleLineForm) => SaleLineForm)}
                {...common}
              />
            )
          }
          if (line.kind === 'full_roll') {
            return (
              <FullRollCard
                key={line.key}
                line={line}
                rolls={intactRolls}
                rollById={rollById}
                productById={productById}
                onChange={(change) => updateLine(line.key, change as (item: SaleLineForm) => SaleLineForm)}
                {...common}
              />
            )
          }
          return (
            <TableclothCard
              key={line.key}
              line={line}
              tablecloths={sellableCloths}
              clothById={clothById}
              productById={productById}
              onChange={(change) => updateLine(line.key, change as (item: SaleLineForm) => SaleLineForm)}
              {...common}
            />
          )
        })}

        <div className="form-actions">
          <button type="button" className="button" disabled={rolls.length === 0} onClick={() => addLine(newRollMetersLine())}>
            {t.addMeters}
          </button>
          <button type="button" className="button" disabled={intactRolls.length === 0} onClick={() => addLine(newFullRollLine())}>
            {t.addFullRoll}
          </button>
          <button type="button" className="button" disabled={sellableCloths.length === 0} onClick={() => addLine(newTableclothLine())}>
            {t.addTablecloth}
          </button>
        </div>
        {/* A disabled button must say why, and what to do about it. */}
        {(rolls.length === 0 || intactRolls.length === 0 || sellableCloths.length === 0) && (
          <ul className="muted unavailable-notes">
            {rolls.length === 0 && <li>{t.noRolls}</li>}
            {rolls.length > 0 && intactRolls.length === 0 && <li>{t.noIntactRolls}</li>}
            {sellableCloths.length === 0 && <li>{t.noTablecloths}</li>}
            <li>
              <Link to="/compras/nueva">{es.purchases.newPurchase}</Link>
            </li>
          </ul>
        )}
      </fieldset>

      <section className="summary" aria-label={t.sections.summary}>
        <h2>{t.sections.summary}</h2>
        <dl>
          <div className="summary-total">
            <dt>{t.summary.total}</dt>
            <dd>{es.sales.money(formatDecimalEs(total))}</dd>
          </div>
        </dl>
        <p className="field-hint">{t.summary.hint}</p>
      </section>

      <div className="form-actions">
        <button type="submit" className="button button-primary" disabled={submitting}>
          {submitting ? t.submitting : t.submit}
        </button>
        <button type="button" className="button" onClick={onCancel} disabled={submitting}>
          {es.common.cancel}
        </button>
      </div>
    </form>
  )
}

interface CardProps<L> {
  line: L
  index: number
  errors: Errors
  onChange: (change: (line: L) => L) => void
  onRemove: () => void
  clearError: (path: string) => void
}

function lineTotalText(line: SaleLineForm): string | null {
  const total = estimateLineTotal(line)
  return total === null ? null : t.lineTotal(formatDecimalEs(hundredthsToApi(total)))
}

function rollOptions(rolls: Roll[]) {
  return rolls.map((roll) => ({
    value: roll.id,
    label: t.rollOption(roll.code, roll.product.sku, formatDecimalEs(roll.available_length_m)),
  }))
}

interface RollCardProps<L extends RollMetersLine | FullRollLine> extends CardProps<L> {
  rolls: Roll[]
  rollById: Map<string, Roll>
  productById: Map<string, Product>
}

function RollMetersCard({ line, index, rolls, rollById, productById, errors, onChange, onRemove, clearError }: RollCardProps<RollMetersLine>) {
  const prefix = `lines[${index}]`
  const number = index + 1
  const roll = rollById.get(line.rollId)
  const reference = roll ? productById.get(roll.product.id)?.pvc?.reference_price_per_meter : undefined

  return (
    <section className="line-card" aria-label={t.lineTitle(number, es.sales.kinds.roll_meters)}>
      <header className="line-card-header">
        <h3>{t.lineTitle(number, es.sales.kinds.roll_meters)}</h3>
        <IconButton label={t.removeLine(number)} icon={<TrashIcon />} variant="danger" onClick={onRemove} />
      </header>

      <SelectField
        id={`sale-${line.key}-roll`}
        path={`${prefix}.roll_id`}
        label={t.rollLabel}
        placeholder={t.rollPlaceholder}
        value={line.rollId}
        options={rollOptions(rolls)}
        error={errors[`${prefix}.roll_id`]}
        onChange={(value) => {
          const chosen = rollById.get(value)
          const price = chosen ? productById.get(chosen.product.id)?.pvc?.reference_price_per_meter : undefined
          // Suggest the catalog price when nothing has been typed yet.
          onChange((current) => ({ ...current, rollId: value, unitPrice: current.unitPrice || (price ? toInputDecimal(price) : '') }))
          clearError(`${prefix}.roll_id`)
        }}
      />
      <div className="form-grid">
        <TextField
          id={`sale-${line.key}-meters`}
          path={`${prefix}.meters`}
          label={t.meters}
          inputMode="decimal"
          hint={roll ? t.metersHint(formatDecimalEs(roll.available_length_m)) : undefined}
          value={line.meters}
          error={errors[`${prefix}.meters`]}
          onChange={(value) => {
            onChange((current) => ({ ...current, meters: value }))
            clearError(`${prefix}.meters`)
          }}
        />
        <TextField
          id={`sale-${line.key}-price`}
          path={`${prefix}.unit_price`}
          label={t.pricePerMeter}
          inputMode="decimal"
          hint={reference ? t.referencePrice(formatDecimalEs(reference)) : undefined}
          value={line.unitPrice}
          error={errors[`${prefix}.unit_price`]}
          onChange={(value) => {
            onChange((current) => ({ ...current, unitPrice: value }))
            clearError(`${prefix}.unit_price`)
          }}
        />
      </div>
      <LineTotal line={line} />
    </section>
  )
}

function FullRollCard({ line, index, rolls, rollById, productById, errors, onChange, onRemove, clearError }: RollCardProps<FullRollLine>) {
  const prefix = `lines[${index}]`
  const number = index + 1
  const roll = rollById.get(line.rollId)
  const reference = roll ? productById.get(roll.product.id)?.pvc?.reference_price_per_roll : undefined

  return (
    <section className="line-card" aria-label={t.lineTitle(number, es.sales.kinds.full_roll)}>
      <header className="line-card-header">
        <h3>{t.lineTitle(number, es.sales.kinds.full_roll)}</h3>
        <IconButton label={t.removeLine(number)} icon={<TrashIcon />} variant="danger" onClick={onRemove} />
      </header>

      <SelectField
        id={`sale-${line.key}-roll`}
        path={`${prefix}.roll_id`}
        label={t.rollLabel}
        placeholder={t.rollPlaceholder}
        value={line.rollId}
        options={rollOptions(rolls)}
        hint={t.intactHint}
        error={errors[`${prefix}.roll_id`]}
        onChange={(value) => {
          const chosen = rollById.get(value)
          const price = chosen ? productById.get(chosen.product.id)?.pvc?.reference_price_per_roll : undefined
          onChange((current) => ({ ...current, rollId: value, unitPrice: current.unitPrice || (price ? toInputDecimal(price) : '') }))
          clearError(`${prefix}.roll_id`)
        }}
      />
      <TextField
        id={`sale-${line.key}-price`}
        path={`${prefix}.unit_price`}
        label={t.pricePerRoll}
        inputMode="decimal"
        hint={reference ? t.referencePrice(formatDecimalEs(reference)) : undefined}
        value={line.unitPrice}
        error={errors[`${prefix}.unit_price`]}
        onChange={(value) => {
          onChange((current) => ({ ...current, unitPrice: value }))
          clearError(`${prefix}.unit_price`)
        }}
      />
      <LineTotal line={line} />
    </section>
  )
}

interface TableclothCardProps extends CardProps<TableclothLine> {
  tablecloths: StockItem[]
  clothById: Map<string, StockItem>
  productById: Map<string, Product>
}

function TableclothCard({ line, index, tablecloths, clothById, productById, errors, onChange, onRemove, clearError }: TableclothCardProps) {
  const prefix = `lines[${index}]`
  const number = index + 1
  const stock = clothById.get(line.productId)
  const reference = productById.get(line.productId)?.tablecloth?.unit_price

  return (
    <section className="line-card" aria-label={t.lineTitle(number, es.sales.kinds.tablecloth_units)}>
      <header className="line-card-header">
        <h3>{t.lineTitle(number, es.sales.kinds.tablecloth_units)}</h3>
        <IconButton label={t.removeLine(number)} icon={<TrashIcon />} variant="danger" onClick={onRemove} />
      </header>

      <SelectField
        id={`sale-${line.key}-product`}
        path={`${prefix}.product_id`}
        label={t.productLabel}
        placeholder={t.productPlaceholder}
        value={line.productId}
        options={tablecloths.map((item) => ({
          value: item.product_id,
          label: t.productOption(item.sku, item.name, formatUnitsEs(item.quantity)),
        }))}
        error={errors[`${prefix}.product_id`]}
        onChange={(value) => {
          const price = productById.get(value)?.tablecloth?.unit_price
          onChange((current) => ({ ...current, productId: value, unitPrice: current.unitPrice || (price ? toInputDecimal(price) : '') }))
          clearError(`${prefix}.product_id`)
        }}
      />
      <div className="form-grid">
        <TextField
          id={`sale-${line.key}-quantity`}
          path={`${prefix}.quantity`}
          label={t.quantity}
          inputMode="decimal"
          hint={stock ? t.quantityHint(formatUnitsEs(stock.quantity)) : undefined}
          value={line.quantity}
          error={errors[`${prefix}.quantity`]}
          onChange={(value) => {
            onChange((current) => ({ ...current, quantity: value }))
            clearError(`${prefix}.quantity`)
          }}
        />
        <TextField
          id={`sale-${line.key}-price`}
          path={`${prefix}.unit_price`}
          label={t.pricePerUnit}
          inputMode="decimal"
          hint={reference ? t.referencePrice(formatDecimalEs(reference)) : undefined}
          value={line.unitPrice}
          error={errors[`${prefix}.unit_price`]}
          onChange={(value) => {
            onChange((current) => ({ ...current, unitPrice: value }))
            clearError(`${prefix}.unit_price`)
          }}
        />
      </div>
      <LineTotal line={line} />
    </section>
  )
}

function LineTotal({ line }: { line: SaleLineForm }) {
  const text = lineTotalText(line)
  return text ? <p className="line-total">{text}</p> : null
}
