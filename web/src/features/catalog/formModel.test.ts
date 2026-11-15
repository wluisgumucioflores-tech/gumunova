import { describe, expect, it } from 'vitest'
import { pvcProduct, tableclothProduct } from '../../test/fixtures'
import { emptyValues, fieldsFor, toProductInput, validate, valuesFromProduct, type FormValues } from './formModel'

function validPvcValues(): FormValues {
  return {
    ...emptyValues(),
    sku: 'pvc-floral-20',
    name: 'PVC floral 20 m',
    'pvc.width_m': '1,40',
    'pvc.nominal_length_m': '20',
    'pvc.reference_price_per_meter': '15,5',
    'pvc.reference_price_per_roll': '280',
  }
}

describe('validate (pvc)', () => {
  it('accepts a complete form', () => {
    expect(validate('pvc', validPvcValues())).toEqual({})
  })

  it('requires SKU, name and the four mandatory numbers', () => {
    expect(validate('pvc', emptyValues())).toEqual({
      sku: 'required',
      name: 'required',
      'pvc.width_m': 'required',
      'pvc.nominal_length_m': 'required',
      'pvc.reference_price_per_meter': 'required',
      'pvc.reference_price_per_roll': 'required',
    })
  })

  it('rejects malformed SKUs', () => {
    const check = (sku: string) => validate('pvc', { ...validPvcValues(), sku }).sku
    expect(check('ab')).toBe('invalid_length')
    expect(check('x'.repeat(41))).toBe('invalid_length')
    expect(check('PVC FLORAL')).toBe('invalid_format')
    expect(check('-PVC-01')).toBe('invalid_format')
    expect(check('PVC-ÑANDÚ')).toBe('invalid_format')
    expect(check('PVC_floral-01')).toBeUndefined()
  })

  it('rejects zero, negative, malformed and over-precise decimals', () => {
    const check = (key: keyof FormValues, value: string) => validate('pvc', { ...validPvcValues(), [key]: value })[key]
    expect(check('pvc.width_m', '0')).toBe('must_be_positive')
    expect(check('pvc.width_m', '0,00')).toBe('must_be_positive')
    expect(check('pvc.width_m', '-1')).toBe('invalid_decimal')
    expect(check('pvc.width_m', 'abc')).toBe('invalid_decimal')
    expect(check('pvc.width_m', '1,234')).toBe('invalid_decimal')
  })

  it('treats thickness as optional but positive when present', () => {
    const check = (value: string) => validate('pvc', { ...validPvcValues(), 'pvc.thickness_mm': value })['pvc.thickness_mm']
    expect(check('')).toBeUndefined()
    expect(check('0,3')).toBeUndefined()
    expect(check('0')).toBe('must_be_positive')
  })

  it('limits free text length', () => {
    const values = { ...validPvcValues(), 'pvc.color': 'x'.repeat(81), name: 'x'.repeat(121) }
    const problems = validate('pvc', values)
    expect(problems['pvc.color']).toBe('too_long')
    expect(problems.name).toBe('too_long')
  })

  it('ignores fields that belong to the other product type', () => {
    const values = { ...validPvcValues(), 'tablecloth.unit_price': 'not a number' }
    expect(validate('pvc', values)).toEqual({})
  })
})

describe('validate (tablecloth)', () => {
  it('requires only the unit price besides SKU and name', () => {
    const problems = validate('tablecloth', emptyValues())
    expect(problems).toEqual({ sku: 'required', name: 'required', 'tablecloth.unit_price': 'required' })
  })

  it('allows a zero reference cost but not a malformed one', () => {
    const base = { ...emptyValues(), sku: 'MAN-01', name: 'Mantel', 'tablecloth.unit_price': '60' }
    expect(validate('tablecloth', { ...base, 'tablecloth.reference_cost': '0' })).toEqual({})
    expect(validate('tablecloth', { ...base, 'tablecloth.reference_cost': 'x' })['tablecloth.reference_cost']).toBe(
      'invalid_decimal',
    )
  })
})

describe('toProductInput', () => {
  it('builds the PVC payload with canonical decimals and null for blank optionals', () => {
    expect(toProductInput('pvc', { ...validPvcValues(), sku: ' PVC-01 ', name: ' PVC ' })).toEqual({
      sku: 'PVC-01',
      name: 'PVC',
      product_type: 'pvc',
      pvc: {
        material: '',
        design: '',
        color: '',
        texture: '',
        width_m: '1.40',
        nominal_length_m: '20.00',
        thickness_mm: null,
        reference_price_per_meter: '15.50',
        reference_price_per_roll: '280.00',
        minimum_stock_m: '0.00',
      },
    })
  })

  it('sends thickness when given', () => {
    const input = toProductInput('pvc', { ...validPvcValues(), 'pvc.thickness_mm': '0,3' })
    expect(input.pvc?.thickness_mm).toBe('0.30')
  })

  it('builds the tablecloth payload without PVC data', () => {
    const input = toProductInput('tablecloth', {
      ...emptyValues(),
      sku: 'MAN-01',
      name: 'Mantel',
      'tablecloth.dimensions': '1,40 x 2,00 m',
      'tablecloth.unit_price': '60',
      'tablecloth.reference_cost': '40,25',
    })
    expect(input.pvc).toBeUndefined()
    expect(input.tablecloth).toEqual({
      design: '',
      color: '',
      dimensions: '1,40 x 2,00 m',
      reference_cost: '40.25',
      unit_price: '60.00',
      minimum_units: 0,
    })
  })
})

describe('valuesFromProduct', () => {
  it('shows decimals with the Spanish separator', () => {
    const values = valuesFromProduct(pvcProduct())
    expect(values['pvc.width_m']).toBe('1,40')
    expect(values['pvc.reference_price_per_meter']).toBe('15,50')
    expect(values.sku).toBe('PVC-FLORAL-20')
  })

  it('round-trips through toProductInput without changing the numbers', () => {
    const original = pvcProduct()
    const input = toProductInput('pvc', valuesFromProduct(original))
    expect(input.pvc?.width_m).toBe(original.pvc?.width_m)
    expect(input.pvc?.reference_price_per_roll).toBe(original.pvc?.reference_price_per_roll)
  })

  it('maps tablecloth data', () => {
    const values = valuesFromProduct(tableclothProduct())
    expect(values['tablecloth.unit_price']).toBe('60,00')
    expect(values['tablecloth.reference_cost']).toBe('')
  })
})

describe('fieldsFor', () => {
  it('returns only fields of the chosen type plus the common ones', () => {
    expect(fieldsFor('pvc').every((key) => !key.startsWith('tablecloth.'))).toBe(true)
    expect(fieldsFor('tablecloth').every((key) => !key.startsWith('pvc.'))).toBe(true)
    expect(fieldsFor('pvc')).toContain('sku')
    expect(fieldsFor('tablecloth')).toContain('name')
  })
})

describe('minimum stock', () => {
  it('is optional and sent as zero when left empty', () => {
    expect(validate('pvc', validPvcValues())).toEqual({})
    expect(toProductInput('pvc', validPvcValues()).pvc?.minimum_stock_m).toBe('0.00')
    const cloth = { ...emptyValues(), sku: 'MAN-01', name: 'Mantel', 'tablecloth.unit_price': '60' }
    expect(toProductInput('tablecloth', cloth).tablecloth?.minimum_units).toBe(0)
  })

  it('accepts meters with decimals for PVC and whole units for tablecloths', () => {
    const pvc = { ...validPvcValues(), 'pvc.minimum_stock_m': '50,5' }
    expect(validate('pvc', pvc)).toEqual({})
    expect(toProductInput('pvc', pvc).pvc?.minimum_stock_m).toBe('50.50')

    const cloth = { ...emptyValues(), sku: 'MAN-01', name: 'Mantel', 'tablecloth.unit_price': '60', 'tablecloth.minimum_units': '8' }
    expect(validate('tablecloth', cloth)).toEqual({})
    expect(toProductInput('tablecloth', cloth).tablecloth?.minimum_units).toBe(8)
  })

  it('rejects malformed or negative values', () => {
    expect(validate('pvc', { ...validPvcValues(), 'pvc.minimum_stock_m': 'abc' })['pvc.minimum_stock_m']).toBe('invalid_decimal')
    const cloth = { ...emptyValues(), sku: 'MAN-01', name: 'Mantel', 'tablecloth.unit_price': '60' }
    for (const bad of ['2,5', '-1', 'x']) {
      expect(validate('tablecloth', { ...cloth, 'tablecloth.minimum_units': bad })['tablecloth.minimum_units']).toBe('invalid_decimal')
    }
    expect(validate('tablecloth', { ...cloth, 'tablecloth.minimum_units': '100001' })['tablecloth.minimum_units']).toBe('out_of_range')
  })

  it('shows an empty field when there is no alert, and the value when there is', () => {
    expect(valuesFromProduct(pvcProduct())['pvc.minimum_stock_m']).toBe('')
    expect(valuesFromProduct(tableclothProduct())['tablecloth.minimum_units']).toBe('')
    const withMinimum = pvcProduct({ pvc: { ...pvcProduct().pvc!, minimum_stock_m: '50.50' } })
    expect(valuesFromProduct(withMinimum)['pvc.minimum_stock_m']).toBe('50,50')
  })
})

