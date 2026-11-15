import type { SaleInput, SaleLineInput } from '../../api/sales'
import { toApiDecimal } from '../../lib/decimal'
import { hundredthsToApi, multiplyHundredths, toHundredths } from '../../lib/money'

// The form keeps every value as the text the person typed. Errors are keyed by the same paths the
// API uses ("lines[0].meters"), so server answers map straight onto inputs.

export interface RollMetersLine {
  kind: 'roll_meters'
  key: string
  rollId: string
  meters: string
  unitPrice: string
}

export interface FullRollLine {
  kind: 'full_roll'
  key: string
  rollId: string
  unitPrice: string
}

export interface TableclothLine {
  kind: 'tablecloth_units'
  key: string
  productId: string
  quantity: string
  unitPrice: string
}

export type SaleLineForm = RollMetersLine | FullRollLine | TableclothLine

export interface SaleFormValues {
  saleDate: string // YYYY-MM-DD
  notes: string
  lines: SaleLineForm[]
}

const MAX_LINES = 100
const MAX_UNITS = 100_000
const MAX_NOTES = 500
const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/

let keyCounter = 0
/** A stable identity for a row, so React keeps its inputs when rows are added or removed. */
export function newLineKey(): string {
  keyCounter += 1
  return `sale-line-${keyCounter}`
}

export function newRollMetersLine(): RollMetersLine {
  return { kind: 'roll_meters', key: newLineKey(), rollId: '', meters: '', unitPrice: '' }
}

export function newFullRollLine(): FullRollLine {
  return { kind: 'full_roll', key: newLineKey(), rollId: '', unitPrice: '' }
}

export function newTableclothLine(): TableclothLine {
  return { kind: 'tablecloth_units', key: newLineKey(), productId: '', quantity: '', unitPrice: '' }
}

/** Today as YYYY-MM-DD in the browser's local time zone. */
export function todayAsInputDate(now: Date = new Date()): string {
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${now.getFullYear()}-${month}-${day}`
}

export function emptySale(today: string = todayAsInputDate()): SaleFormValues {
  return { saleDate: today, notes: '', lines: [] }
}

/** Codes understood by the Spanish catalog in i18n/es.ts (fieldErrors). */
export type SaleValidationCode =
  | 'required'
  | 'too_long'
  | 'too_many'
  | 'invalid_format'
  | 'invalid_decimal'
  | 'must_be_positive'
  | 'out_of_range'

/**
 * Quick checks that save a round trip. The backend validates everything again, including the
 * stock, and is the authority; its answers are shown the same way.
 */
export function validateSale(values: SaleFormValues): Record<string, SaleValidationCode> {
  const problems: Record<string, SaleValidationCode> = {}

  if (!values.saleDate) problems.sale_date = 'required'
  else if (!DATE_PATTERN.test(values.saleDate)) problems.sale_date = 'invalid_format'
  if (values.notes.trim().length > MAX_NOTES) problems.notes = 'too_long'

  if (values.lines.length === 0) problems.lines = 'required'
  else if (values.lines.length > MAX_LINES) problems.lines = 'too_many'

  values.lines.forEach((line, index) => {
    const prefix = `lines[${index}]`
    const price = positiveProblem(line.unitPrice)
    if (price) problems[`${prefix}.unit_price`] = price

    if (line.kind === 'tablecloth_units') {
      if (!line.productId) problems[`${prefix}.product_id`] = 'required'
      const quantity = line.quantity.trim()
      if (!quantity) problems[`${prefix}.quantity`] = 'required'
      else if (!/^\d+$/.test(quantity)) problems[`${prefix}.quantity`] = 'invalid_decimal'
      else if (Number(quantity) < 1) problems[`${prefix}.quantity`] = 'must_be_positive'
      else if (Number(quantity) > MAX_UNITS) problems[`${prefix}.quantity`] = 'out_of_range'
      return
    }

    if (!line.rollId) problems[`${prefix}.roll_id`] = 'required'
    if (line.kind === 'roll_meters') {
      const meters = positiveProblem(line.meters)
      if (meters) problems[`${prefix}.meters`] = meters
    }
  })

  return problems
}

function positiveProblem(text: string): SaleValidationCode | null {
  const trimmed = text.trim()
  if (!trimmed) return 'required'
  const normalized = toApiDecimal(trimmed)
  if (normalized === null) return 'invalid_decimal'
  return normalized === '0.00' ? 'must_be_positive' : null
}

/** Estimated total of one line, in hundredths (null while its numbers are not readable). */
export function estimateLineTotal(line: SaleLineForm): bigint | null {
  const price = toHundredths(line.unitPrice)
  if (price === null) return null
  switch (line.kind) {
    case 'roll_meters': {
      const meters = toHundredths(line.meters)
      return meters === null ? null : multiplyHundredths(price, meters)
    }
    case 'full_roll':
      return price
    case 'tablecloth_units': {
      const units = line.quantity.trim()
      return /^\d+$/.test(units) ? multiplyHundredths(price, BigInt(units) * 100n) : null
    }
  }
}

/** Estimated total of the sale as an exact decimal string. Unreadable lines count as zero. */
export function estimateTotal(values: SaleFormValues): string {
  let total = 0n
  for (const line of values.lines) total += estimateLineTotal(line) ?? 0n
  return hundredthsToApi(total)
}

/** Builds the API payload. Call only after validateSale found no problems. */
export function toSaleInput(values: SaleFormValues): SaleInput {
  const decimal = (text: string) => toApiDecimal(text) ?? ''
  return {
    sale_date: values.saleDate,
    notes: values.notes.trim(),
    lines: values.lines.map((line): SaleLineInput => {
      switch (line.kind) {
        case 'roll_meters':
          return { kind: line.kind, roll_id: line.rollId, meters: decimal(line.meters), unit_price: decimal(line.unitPrice) }
        case 'full_roll':
          return { kind: line.kind, roll_id: line.rollId, unit_price: decimal(line.unitPrice) }
        case 'tablecloth_units':
          return { kind: line.kind, product_id: line.productId, quantity: Number(line.quantity.trim()), unit_price: decimal(line.unitPrice) }
      }
    }),
  }
}

/** A random identifier for one registration attempt. */
export function newIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}
