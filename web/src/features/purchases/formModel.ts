import type { Purchase, PurchaseInput, PvcLineInput, TableclothLineInput } from '../../api/purchases'
import { emptyValues, toProductInput, validate as validateProduct, type FormValues as ProductFormValues } from '../catalog/formModel'
import { toApiDecimal, toInputDecimal } from '../../lib/decimal'
import { hundredthsToApi, toHundredths } from '../../lib/money'

// The form keeps every value as the text the person typed. Errors are keyed by the same paths the
// API uses ("lines[0].rolls[1].length_m"), so server answers map straight onto inputs.

export interface RollRow {
  key: string
  length: string
  cost: string
}

/** Value of the product selector that asks for a product that is not in the catalog yet. */
export const NEW_PRODUCT = '__new__'

export interface PvcLine {
  kind: 'pvc'
  key: string
  productId: string
  /** Set instead of productId when the product is created together with the purchase. */
  newProduct: ProductFormValues | null
  rolls: RollRow[]
}

export interface TableclothLine {
  kind: 'tablecloth'
  key: string
  productId: string
  newProduct: ProductFormValues | null
  quantity: string
  unitCost: string
}

export type LineForm = PvcLine | TableclothLine

export interface PurchaseFormValues {
  supplierId: string
  purchaseDate: string // YYYY-MM-DD
  documentReference: string
  notes: string
  additionalCosts: string
  lines: LineForm[]
}

const MAX_LINES = 100
/** The most rolls a line may hold. */
export const MAX_ROLLS = 100
const MAX_QUANTITY = 100_000
const MAX_REFERENCE = 60
const MAX_NOTES = 500
const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/

let keyCounter = 0
/** A stable identity for a row, so React keeps its inputs when rows are added or removed. */
export function newRowKey(): string {
  keyCounter += 1
  return `row-${keyCounter}`
}

export function newRoll(copyFrom?: RollRow): RollRow {
  return { key: newRowKey(), length: copyFrom?.length ?? '', cost: copyFrom?.cost ?? '' }
}

/**
 * Resizes the rolls of a line to `count`. New rolls copy the last one (the same model is usually
 * bought with the same length and cost); removing keeps the first ones.
 */
export function resizeRolls(rolls: RollRow[], count: number): RollRow[] {
  const target = Math.min(Math.max(count, 1), MAX_ROLLS)
  if (target <= rolls.length) return rolls.slice(0, target)
  const grown = [...rolls]
  while (grown.length < target) grown.push(newRoll(grown[grown.length - 1]))
  return grown
}

export function newPvcLine(): PvcLine {
  return { kind: 'pvc', key: newRowKey(), productId: '', newProduct: null, rolls: [newRoll()] }
}

export function newTableclothLine(): TableclothLine {
  return { kind: 'tablecloth', key: newRowKey(), productId: '', newProduct: null, quantity: '', unitCost: '' }
}

/** Switches a line between an existing product and a new one, keeping the other fields. */
export function withProductChoice<L extends LineForm>(line: L, choice: string): L {
  if (choice === NEW_PRODUCT) return { ...line, productId: '', newProduct: line.newProduct ?? emptyValues() }
  return { ...line, productId: choice, newProduct: null }
}

/** Today as YYYY-MM-DD in the browser's local time zone. */
export function todayAsInputDate(now: Date = new Date()): string {
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${now.getFullYear()}-${month}-${day}`
}

export function emptyPurchase(today: string = todayAsInputDate()): PurchaseFormValues {
  return { supplierId: '', purchaseDate: today, documentReference: '', notes: '', additionalCosts: '', lines: [] }
}

/** The form values that reproduce a stored purchase, ready to be corrected. */
export function purchaseToFormValues(purchase: Purchase): PurchaseFormValues {
  const hasAdditional = purchase.additional_costs !== '0.00'
  return {
    supplierId: purchase.supplier?.id ?? '',
    purchaseDate: purchase.purchase_date,
    documentReference: purchase.document_reference,
    notes: purchase.notes,
    additionalCosts: hasAdditional ? toInputDecimal(purchase.additional_costs) : '',
    lines: purchase.lines.map((line): LineForm => {
      const base = { key: newRowKey(), productId: line.product.id, newProduct: null }
      if (line.product.product_type === 'pvc') {
        return {
          ...base,
          kind: 'pvc',
          rolls: line.rolls.map((roll) => ({
            key: newRowKey(),
            length: toInputDecimal(roll.length_m),
            cost: toInputDecimal(roll.acquisition_cost),
          })),
        }
      }
      return { ...base, kind: 'tablecloth', quantity: String(line.quantity), unitCost: toInputDecimal(line.unit_cost ?? '') }
    }),
  }
}

/** Codes understood by the Spanish catalog in i18n/es.ts (fieldErrors). */
export type PurchaseValidationCode =
  | 'required'
  | 'too_long'
  | 'too_many'
  | 'invalid_format'
  | 'invalid_decimal'
  | 'must_be_positive'
  | 'out_of_range'
  | 'invalid_length'
  | 'must_not_be_negative'

/**
 * Quick checks that save a round trip. The backend validates everything again and is the
 * authority; its answers are shown the same way.
 */
export function validatePurchase(values: PurchaseFormValues): Record<string, PurchaseValidationCode> {
  const problems: Record<string, PurchaseValidationCode> = {}

  if (!values.purchaseDate) problems.purchase_date = 'required'
  else if (!DATE_PATTERN.test(values.purchaseDate)) problems.purchase_date = 'invalid_format'

  if (values.documentReference.trim().length > MAX_REFERENCE) problems.document_reference = 'too_long'
  if (values.notes.trim().length > MAX_NOTES) problems.notes = 'too_long'

  const additional = values.additionalCosts.trim()
  if (additional && toApiDecimal(additional) === null) problems.additional_costs = 'invalid_decimal'

  if (values.lines.length === 0) problems.lines = 'required'
  else if (values.lines.length > MAX_LINES) problems.lines = 'too_many'

  values.lines.forEach((line, index) => {
    const prefix = `lines[${index}]`
    if (line.newProduct) {
      for (const [key, code] of Object.entries(validateProduct(line.kind, line.newProduct))) {
        problems[`${prefix}.new_product.${key}`] = code
      }
    } else if (!line.productId) {
      problems[`${prefix}.product_id`] = 'required'
    }

    if (line.kind === 'pvc') {
      if (line.rolls.length === 0) problems[`${prefix}.rolls`] = 'required'
      else if (line.rolls.length > MAX_ROLLS) problems[`${prefix}.rolls`] = 'too_many'
      line.rolls.forEach((roll, rollIndex) => {
        const rollPrefix = `${prefix}.rolls[${rollIndex}]`
        const length = positiveProblem(roll.length)
        if (length) problems[`${rollPrefix}.length_m`] = length
        const cost = positiveProblem(roll.cost)
        if (cost) problems[`${rollPrefix}.cost`] = cost
      })
      return
    }

    const quantity = line.quantity.trim()
    if (!quantity) problems[`${prefix}.quantity`] = 'required'
    else if (!/^\d+$/.test(quantity)) problems[`${prefix}.quantity`] = 'invalid_decimal'
    else if (Number(quantity) < 1) problems[`${prefix}.quantity`] = 'must_be_positive'
    else if (Number(quantity) > MAX_QUANTITY) problems[`${prefix}.quantity`] = 'out_of_range'

    const unitCost = positiveProblem(line.unitCost)
    if (unitCost) problems[`${prefix}.unit_cost`] = unitCost
  })

  return problems
}

function positiveProblem(text: string): PurchaseValidationCode | null {
  const trimmed = text.trim()
  if (!trimmed) return 'required'
  const normalized = toApiDecimal(trimmed)
  if (normalized === null) return 'invalid_decimal'
  return normalized === '0.00' ? 'must_be_positive' : null
}

export interface PurchaseTotals {
  /** Cost of everything bought, before additional costs. */
  goods: string
  additional: string
  total: string
}

/** Estimated totals, as exact decimal strings. Unreadable inputs count as zero. */
export function estimateTotals(values: PurchaseFormValues): PurchaseTotals {
  let goods = 0n
  for (const line of values.lines) {
    if (line.kind === 'pvc') {
      for (const roll of line.rolls) goods += toHundredths(roll.cost) ?? 0n
    } else {
      const quantity = /^\d+$/.test(line.quantity.trim()) ? BigInt(line.quantity.trim()) : 0n
      goods += quantity * (toHundredths(line.unitCost) ?? 0n)
    }
  }
  const additional = toHundredths(values.additionalCosts) ?? 0n
  return {
    goods: hundredthsToApi(goods),
    additional: hundredthsToApi(additional),
    total: hundredthsToApi(goods + additional),
  }
}

/** Builds the API payload. Call only after validatePurchase found no problems. */
export function toPurchaseInput(values: PurchaseFormValues): PurchaseInput {
  const decimal = (text: string) => toApiDecimal(text) ?? ''
  return {
    supplier_id: values.supplierId,
    purchase_date: values.purchaseDate,
    document_reference: values.documentReference.trim(),
    notes: values.notes.trim(),
    additional_costs: decimal(values.additionalCosts) || '0.00',
    lines: values.lines.map((line): PvcLineInput | TableclothLineInput => {
      const product = line.newProduct
        ? { new_product: toProductInput(line.kind, line.newProduct) }
        : { product_id: line.productId }
      return line.kind === 'pvc'
        ? {
            ...product,
            rolls: line.rolls.map((roll) => ({ length_m: decimal(roll.length), cost: decimal(roll.cost) })),
          }
        : { ...product, quantity: Number(line.quantity.trim()), unit_cost: decimal(line.unitCost) }
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
