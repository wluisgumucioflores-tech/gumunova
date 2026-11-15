import type { Product, ProductInput, ProductType } from '../../api/products'
import { toApiDecimal, toInputDecimal } from '../../lib/decimal'

// Form values are keyed by the same path the API uses in field errors ("pvc.width_m"),
// so server-side validation errors map straight onto inputs.
export const FIELD_KEYS = [
  'sku',
  'name',
  'pvc.material',
  'pvc.design',
  'pvc.color',
  'pvc.texture',
  'pvc.width_m',
  'pvc.nominal_length_m',
  'pvc.thickness_mm',
  'pvc.reference_price_per_meter',
  'pvc.reference_price_per_roll',
  'pvc.minimum_stock_m',
  'tablecloth.design',
  'tablecloth.color',
  'tablecloth.dimensions',
  'tablecloth.reference_cost',
  'tablecloth.unit_price',
  'tablecloth.minimum_units',
] as const

export type FieldKey = (typeof FIELD_KEYS)[number]
export type FormValues = Record<FieldKey, string>
export type FormErrors = Partial<Record<string, string>>

/** Codes understood by the Spanish catalog in i18n/es.ts (fieldErrors). */
export type ValidationCode = 'out_of_range' | 'required' | 'invalid_length' | 'invalid_format' | 'too_long' | 'must_be_positive' | 'invalid_decimal' | 'must_not_be_negative'

const SKU_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]{2,39}$/
const MAX_NAME_LENGTH = 120
const MAX_ATTRIBUTE_LENGTH = 80
const MAX_MINIMUM_UNITS = 100_000

function isZeroDecimal(apiDecimal: string): boolean {
  return /^0+(\.0+)?$/.test(apiDecimal)
}

export function emptyValues(): FormValues {
  return Object.fromEntries(FIELD_KEYS.map((key) => [key, ''])) as FormValues
}

export function valuesFromProduct(product: Product): FormValues {
  const values = emptyValues()
  values.sku = product.sku
  values.name = product.name
  if (product.pvc) {
    values['pvc.material'] = product.pvc.material
    values['pvc.design'] = product.pvc.design
    values['pvc.color'] = product.pvc.color
    values['pvc.texture'] = product.pvc.texture
    values['pvc.width_m'] = toInputDecimal(product.pvc.width_m)
    values['pvc.nominal_length_m'] = toInputDecimal(product.pvc.nominal_length_m)
    values['pvc.thickness_mm'] = product.pvc.thickness_mm ? toInputDecimal(product.pvc.thickness_mm) : ''
    values['pvc.reference_price_per_meter'] = toInputDecimal(product.pvc.reference_price_per_meter)
    values['pvc.reference_price_per_roll'] = toInputDecimal(product.pvc.reference_price_per_roll)
    values['pvc.minimum_stock_m'] = isZeroDecimal(product.pvc.minimum_stock_m) ? '' : toInputDecimal(product.pvc.minimum_stock_m)
  }
  if (product.tablecloth) {
    values['tablecloth.design'] = product.tablecloth.design
    values['tablecloth.color'] = product.tablecloth.color
    values['tablecloth.dimensions'] = product.tablecloth.dimensions
    values['tablecloth.reference_cost'] = product.tablecloth.reference_cost
      ? toInputDecimal(product.tablecloth.reference_cost)
      : ''
    values['tablecloth.unit_price'] = toInputDecimal(product.tablecloth.unit_price)
    values['tablecloth.minimum_units'] = product.tablecloth.minimum_units > 0 ? String(product.tablecloth.minimum_units) : ''
  }
  return values
}

/** Fields that apply to a product type, in display order. */
export function fieldsFor(type: ProductType): FieldKey[] {
  const prefix = type === 'pvc' ? 'pvc.' : 'tablecloth.'
  return FIELD_KEYS.filter((key) => key === 'sku' || key === 'name' || key.startsWith(prefix))
}

const REQUIRED_POSITIVE: Record<ProductType, FieldKey[]> = {
  pvc: ['pvc.width_m', 'pvc.nominal_length_m', 'pvc.reference_price_per_meter', 'pvc.reference_price_per_roll'],
  tablecloth: ['tablecloth.unit_price'],
}

const OPTIONAL_POSITIVE: Record<ProductType, FieldKey[]> = {
  pvc: ['pvc.thickness_mm'],
  tablecloth: [],
}

const OPTIONAL_NON_NEGATIVE: Record<ProductType, FieldKey[]> = {
  pvc: ['pvc.minimum_stock_m'],
  tablecloth: ['tablecloth.reference_cost'],
}

const FREE_TEXT: Record<ProductType, FieldKey[]> = {
  pvc: ['pvc.material', 'pvc.design', 'pvc.color', 'pvc.texture'],
  tablecloth: ['tablecloth.design', 'tablecloth.color', 'tablecloth.dimensions'],
}

/**
 * Quick checks that save a round trip. The backend validates everything again and is the
 * authority; its answers are shown the same way.
 */
export function validate(type: ProductType, values: FormValues): Record<string, ValidationCode> {
  const problems: Record<string, ValidationCode> = {}

  const sku = values.sku.trim()
  if (!sku) problems.sku = 'required'
  else if (sku.length < 3 || sku.length > 40) problems.sku = 'invalid_length'
  else if (!SKU_PATTERN.test(sku)) problems.sku = 'invalid_format'

  const name = values.name.trim()
  if (!name) problems.name = 'required'
  else if (name.length > MAX_NAME_LENGTH) problems.name = 'too_long'

  for (const key of FREE_TEXT[type]) {
    if (values[key].trim().length > MAX_ATTRIBUTE_LENGTH) problems[key] = 'too_long'
  }

  for (const key of REQUIRED_POSITIVE[type]) {
    const text = values[key].trim()
    const problem = text ? checkDecimal(text, true) : 'required'
    if (problem) problems[key] = problem
  }
  for (const key of OPTIONAL_POSITIVE[type]) {
    const text = values[key].trim()
    const problem = text ? checkDecimal(text, true) : null
    if (problem) problems[key] = problem
  }
  for (const key of OPTIONAL_NON_NEGATIVE[type]) {
    const text = values[key].trim()
    const problem = text ? checkDecimal(text, false) : null
    if (problem) problems[key] = problem
  }

  if (type === 'tablecloth') {
    const units = values['tablecloth.minimum_units'].trim()
    if (units) {
      if (!/^\d{1,10}$/.test(units)) problems['tablecloth.minimum_units'] = 'invalid_decimal'
      else if (Number(units) > MAX_MINIMUM_UNITS) problems['tablecloth.minimum_units'] = 'out_of_range'
    }
  }

  return problems
}

function checkDecimal(text: string, mustBePositive: boolean): ValidationCode | null {
  const normalized = toApiDecimal(text)
  if (normalized === null) return 'invalid_decimal'
  if (mustBePositive && normalized === '0.00') return 'must_be_positive'
  return null
}

function optionalDecimal(text: string): string | null {
  const trimmed = text.trim()
  return trimmed ? toApiDecimal(trimmed) : null
}

/** Builds the API payload. Call only after validate() found no problems. */
export function toProductInput(type: ProductType, values: FormValues): ProductInput {
  const required = (key: FieldKey): string => toApiDecimal(values[key]) ?? ''

  if (type === 'pvc') {
    return {
      sku: values.sku.trim(),
      name: values.name.trim(),
      product_type: 'pvc',
      pvc: {
        material: values['pvc.material'].trim(),
        design: values['pvc.design'].trim(),
        color: values['pvc.color'].trim(),
        texture: values['pvc.texture'].trim(),
        width_m: required('pvc.width_m'),
        nominal_length_m: required('pvc.nominal_length_m'),
        thickness_mm: optionalDecimal(values['pvc.thickness_mm']),
        reference_price_per_meter: required('pvc.reference_price_per_meter'),
        reference_price_per_roll: required('pvc.reference_price_per_roll'),
        minimum_stock_m: optionalDecimal(values['pvc.minimum_stock_m']) ?? '0.00',
      },
    }
  }
  return {
    sku: values.sku.trim(),
    name: values.name.trim(),
    product_type: 'tablecloth',
    tablecloth: {
      design: values['tablecloth.design'].trim(),
      color: values['tablecloth.color'].trim(),
      dimensions: values['tablecloth.dimensions'].trim(),
      reference_cost: optionalDecimal(values['tablecloth.reference_cost']),
      unit_price: required('tablecloth.unit_price'),
      minimum_units: Number(values['tablecloth.minimum_units'].trim() || '0'),
    },
  }
}
