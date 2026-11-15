import { describe, expect, it } from 'vitest'
import {
  emptySale,
  estimateLineTotal,
  estimateTotal,
  newFullRollLine,
  newIdempotencyKey,
  newRollMetersLine,
  newTableclothLine,
  todayAsInputDate,
  toSaleInput,
  validateSale,
  type SaleFormValues,
} from './formModel'

function validValues(): SaleFormValues {
  const meters = newRollMetersLine()
  meters.rollId = 'roll-1'
  meters.meters = '2,5'
  meters.unitPrice = '18'
  const whole = newFullRollLine()
  whole.rollId = 'roll-2'
  whole.unitPrice = '350'
  const cloth = newTableclothLine()
  cloth.productId = 'cloth-1'
  cloth.quantity = '4'
  cloth.unitPrice = '65,5'
  return { ...emptySale('2026-11-10'), notes: ' nota ', lines: [meters, whole, cloth] }
}

describe('validateSale', () => {
  it('accepts a complete sale of every kind', () => {
    expect(validateSale(validValues())).toEqual({})
  })

  it('needs a date and at least one line', () => {
    expect(validateSale({ ...emptySale(''), saleDate: '' })).toEqual({ sale_date: 'required', lines: 'required' })
    expect(validateSale({ ...emptySale('10/11/2026') }).sale_date).toBe('invalid_format')
  })

  it('names the exact field of an incomplete line', () => {
    const meters = newRollMetersLine()
    const whole = newFullRollLine()
    const cloth = newTableclothLine()

    const problems = validateSale({ ...emptySale('2026-11-10'), lines: [meters, whole, cloth] })

    expect(problems).toEqual({
      'lines[0].roll_id': 'required',
      'lines[0].meters': 'required',
      'lines[0].unit_price': 'required',
      'lines[1].roll_id': 'required',
      'lines[1].unit_price': 'required',
      'lines[2].product_id': 'required',
      'lines[2].quantity': 'required',
      'lines[2].unit_price': 'required',
    })
  })

  it('rejects zero, malformed and fractional values', () => {
    const values = validValues()
    const [meters, , cloth] = values.lines
    if (meters?.kind !== 'roll_meters' || cloth?.kind !== 'tablecloth_units') throw new Error('unexpected lines')
    meters.meters = '0'
    meters.unitPrice = 'abc'
    cloth.quantity = '2,5'

    expect(validateSale(values)).toEqual({
      'lines[0].meters': 'must_be_positive',
      'lines[0].unit_price': 'invalid_decimal',
      'lines[2].quantity': 'invalid_decimal',
    })

    cloth.quantity = '100001'
    expect(validateSale(values)['lines[2].quantity']).toBe('out_of_range')
    cloth.quantity = '0'
    expect(validateSale(values)['lines[2].quantity']).toBe('must_be_positive')
  })

  it('limits the notes', () => {
    expect(validateSale({ ...validValues(), notes: 'x'.repeat(501) }).notes).toBe('too_long')
  })
})

describe('estimateTotal', () => {
  it('multiplies meters by the price per meter and rounds like the server', () => {
    const line = newRollMetersLine()
    line.meters = '1,25'
    line.unitPrice = '15,5'

    expect(estimateLineTotal(line)).toBe(1938n)
  })

  it('takes the price of a whole roll as is and multiplies units by their price', () => {
    const values = validValues()

    expect(estimateLineTotal(values.lines[1]!)).toBe(35000n)
    expect(estimateLineTotal(values.lines[2]!)).toBe(26200n)
    expect(estimateTotal(values)).toBe('657.00') // 45,00 + 350,00 + 262,00
  })

  it('counts unreadable lines as zero and never throws', () => {
    const values = validValues()
    const meters = values.lines[0]!
    if (meters.kind === 'roll_meters') meters.meters = 'xx'

    expect(estimateLineTotal(meters)).toBeNull()
    expect(estimateTotal(values)).toBe('612.00')
    expect(estimateTotal(emptySale('2026-11-10'))).toBe('0.00')
  })
})

describe('toSaleInput', () => {
  it('builds the API payload for each kind with canonical decimals', () => {
    const input = toSaleInput(validValues())

    expect(input).toEqual({
      sale_date: '2026-11-10',
      notes: 'nota',
      lines: [
        { kind: 'roll_meters', roll_id: 'roll-1', meters: '2.50', unit_price: '18.00' },
        { kind: 'full_roll', roll_id: 'roll-2', unit_price: '350.00' },
        { kind: 'tablecloth_units', product_id: 'cloth-1', quantity: 4, unit_price: '65.50' },
      ],
    })
  })
})

describe('helpers', () => {
  it('formats today in local time', () => {
    expect(todayAsInputDate(new Date(2026, 0, 5))).toBe('2026-01-05')
  })

  it('creates a different key every time', () => {
    const keys = new Set(Array.from({ length: 20 }, () => newIdempotencyKey()))
    expect(keys.size).toBe(20)
    for (const key of keys) expect(key).toMatch(/^[A-Za-z0-9._:-]{8,100}$/)
  })
})
