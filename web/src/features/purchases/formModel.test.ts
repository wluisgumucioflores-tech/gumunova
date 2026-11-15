import { describe, expect, it } from 'vitest'
import {
  NEW_PRODUCT,
  emptyPurchase,
  estimateTotals,
  newIdempotencyKey,
  newPvcLine,
  newRoll,
  newTableclothLine,
  resizeRolls,
  todayAsInputDate,
  toPurchaseInput,
  validatePurchase,
  withProductChoice,
  type PurchaseFormValues,
} from './formModel'
import { emptyValues } from '../catalog/formModel'

function validValues(): PurchaseFormValues {
  const pvc = newPvcLine()
  pvc.productId = 'pvc-1'
  pvc.rolls = [
    { ...newRoll(), length: '19,8', cost: '280' },
    { ...newRoll(), length: '20', cost: '300,5' },
  ]
  const cloth = newTableclothLine()
  cloth.productId = 'cloth-1'
  cloth.quantity = '10'
  cloth.unitCost = '30'
  return { ...emptyPurchase('2026-11-10'), supplierId: 'sup-1', additionalCosts: '30', lines: [pvc, cloth] }
}

describe('validatePurchase', () => {
  it('accepts a complete purchase', () => {
    expect(validatePurchase(validValues())).toEqual({})
  })

  it('requires date and at least one line, but not a supplier', () => {
    expect(validatePurchase({ ...emptyPurchase(''), purchaseDate: '' })).toEqual({
      purchase_date: 'required',
      lines: 'required',
    })
  })

  it('rejects a malformed date', () => {
    expect(validatePurchase({ ...validValues(), purchaseDate: '10/11/2026' }).purchase_date).toBe('invalid_format')
  })

  it('reports problems at the exact roll and line, using API paths', () => {
    const values = validValues()
    const pvc = values.lines[0]!
    if (pvc.kind !== 'pvc') throw new Error('fixture')
    pvc.rolls[1] = { ...pvc.rolls[1]!, length: '0', cost: 'abc' }
    pvc.productId = ''

    expect(validatePurchase(values)).toEqual({
      'lines[0].product_id': 'required',
      'lines[0].rolls[1].length_m': 'must_be_positive',
      'lines[0].rolls[1].cost': 'invalid_decimal',
    })
  })

  it('requires every roll to have a length and a cost', () => {
    const values = validValues()
    values.lines = [{ ...newPvcLine(), productId: 'pvc-1' }]

    expect(validatePurchase(values)).toEqual({
      'lines[0].rolls[0].length_m': 'required',
      'lines[0].rolls[0].cost': 'required',
    })
  })

  it('needs rolls on a PVC line, and not too many', () => {
    const values = validValues()
    const pvc = values.lines[0]!
    if (pvc.kind !== 'pvc') throw new Error('fixture')
    pvc.rolls = []
    expect(validatePurchase(values)['lines[0].rolls']).toBe('required')

    pvc.rolls = Array.from({ length: 101 }, () => ({ ...newRoll(), length: '20', cost: '100' }))
    expect(validatePurchase(values)['lines[0].rolls']).toBe('too_many')
  })

  it('validates tablecloth quantity as a whole number within limits', () => {
    const check = (quantity: string) => {
      const values = validValues()
      const cloth = values.lines[1]!
      if (cloth.kind !== 'tablecloth') throw new Error('fixture')
      cloth.quantity = quantity
      return validatePurchase(values)['lines[1].quantity']
    }
    expect(check('')).toBe('required')
    expect(check('1,5')).toBe('invalid_decimal')
    expect(check('2.5')).toBe('invalid_decimal')
    expect(check('0')).toBe('must_be_positive')
    expect(check('100001')).toBe('out_of_range')
    expect(check('100000')).toBeUndefined()
    expect(check('1')).toBeUndefined()
  })

  it('requires a positive unit cost', () => {
    const values = validValues()
    const cloth = values.lines[1]!
    if (cloth.kind !== 'tablecloth') throw new Error('fixture')
    cloth.unitCost = '0'
    expect(validatePurchase(values)['lines[1].unit_cost']).toBe('must_be_positive')
  })

  it('treats additional costs as optional but they must be a valid amount', () => {
    expect(validatePurchase({ ...validValues(), additionalCosts: '' })).toEqual({})
    expect(validatePurchase({ ...validValues(), additionalCosts: '0' })).toEqual({})
    expect(validatePurchase({ ...validValues(), additionalCosts: '-5' }).additional_costs).toBe('invalid_decimal')
    expect(validatePurchase({ ...validValues(), additionalCosts: '1,234' }).additional_costs).toBe('invalid_decimal')
  })

  it('limits free text', () => {
    const problems = validatePurchase({ ...validValues(), documentReference: 'x'.repeat(61), notes: 'x'.repeat(501) })
    expect(problems).toEqual({ document_reference: 'too_long', notes: 'too_long' })
  })
})

describe('estimateTotals', () => {
  it('adds rolls, tablecloth lines and additional costs exactly', () => {
    expect(estimateTotals(validValues())).toEqual({ goods: '880.50', additional: '30.00', total: '910.50' })
  })

  it('is exact where floating point is not', () => {
    const values = validValues()
    values.additionalCosts = ''
    values.lines = [
      { ...newTableclothLine(), productId: 'c', quantity: '3', unitCost: '0,10' },
      { ...newTableclothLine(), productId: 'c', quantity: '3', unitCost: '0,20' },
    ]
    // 3 * 0.10 + 3 * 0.20 = 0.90 (0.1 * 3 is 0.30000000000000004 as a number)
    expect(estimateTotals(values).total).toBe('0.90')
  })

  it('counts unreadable or empty inputs as zero while the form is being filled in', () => {
    const values = validValues()
    values.additionalCosts = 'abc'
    values.lines = [{ ...newTableclothLine(), productId: 'c', quantity: 'x', unitCost: '5' }]
    expect(estimateTotals(values)).toEqual({ goods: '0.00', additional: '0.00', total: '0.00' })
    expect(estimateTotals(emptyPurchase())).toEqual({ goods: '0.00', additional: '0.00', total: '0.00' })
  })
})

describe('toPurchaseInput', () => {
  it('builds the API payload with canonical decimals and kinds taken from the line type', () => {
    const values = validValues()
    values.documentReference = ' FAC-001 '

    expect(toPurchaseInput(values)).toEqual({
      supplier_id: 'sup-1',
      purchase_date: '2026-11-10',
      document_reference: 'FAC-001',
      notes: '',
      additional_costs: '30.00',
      lines: [
        {
          product_id: 'pvc-1',
          rolls: [
            { length_m: '19.80', cost: '280.00' },
            { length_m: '20.00', cost: '300.50' },
          ],
        },
        { product_id: 'cloth-1', quantity: 10, unit_cost: '30.00' },
      ],
    })
  })

  it('sends zero when there are no additional costs', () => {
    expect(toPurchaseInput({ ...validValues(), additionalCosts: '' }).additional_costs).toBe('0.00')
  })
})

describe('helpers', () => {
  it('copies the previous roll so identical rolls are quick to enter', () => {
    const first = { ...newRoll(), length: '20', cost: '100' }
    const copy = newRoll(first)
    expect(copy).toMatchObject({ length: '20', cost: '100' })
    expect(copy.key).not.toBe(first.key)
  })

  it('formats today in local time', () => {
    expect(todayAsInputDate(new Date(2026, 10, 9))).toBe('2026-11-09')
    expect(todayAsInputDate(new Date(2026, 0, 31))).toBe('2026-01-31')
  })

  it('creates a different idempotency key every time, long enough for the API', () => {
    const first = newIdempotencyKey()
    expect(first).not.toBe(newIdempotencyKey())
    expect(first).toMatch(/^[A-Za-z0-9._:-]{8,100}$/)
  })
})

describe('lines that create their product', () => {
  function withNewPvc(): PurchaseFormValues {
    const values = validValues()
    const line = values.lines[0]!
    if (line.kind !== 'pvc') throw new Error('expected a PVC line')
    values.lines[0] = withProductChoice(line, NEW_PRODUCT)
    return values
  }

  it('switching to a new product clears the chosen product and back again keeps the rolls', () => {
    const line = newPvcLine()
    line.productId = 'pvc-1'
    line.rolls = [{ ...newRoll(), length: '10', cost: '100' }]

    const asNew = withProductChoice(line, NEW_PRODUCT)
    expect(asNew.productId).toBe('')
    expect(asNew.newProduct).toEqual(emptyValues())

    const back = withProductChoice(asNew, 'pvc-2')
    expect(back.productId).toBe('pvc-2')
    expect(back.newProduct).toBeNull()
    expect(back.rolls).toEqual(line.rolls)
  })

  it('validates the new product instead of asking for a product id', () => {
    const problems = validatePurchase(withNewPvc())

    expect(problems['lines[0].product_id']).toBeUndefined()
    expect(problems['lines[0].new_product.sku']).toBe('required')
    expect(problems['lines[0].new_product.name']).toBe('required')
    expect(problems['lines[0].new_product.pvc.width_m']).toBe('required')
    expect(problems['lines[0].new_product.pvc.reference_price_per_roll']).toBe('required')
  })

  it('sends new_product instead of product_id once it is complete', () => {
    const values = withNewPvc()
    const line = values.lines[0]!
    line.newProduct = {
      ...emptyValues(),
      sku: 'PVC-NUEVO',
      name: 'PVC nuevo',
      'pvc.width_m': '1,4',
      'pvc.nominal_length_m': '20',
      'pvc.reference_price_per_meter': '15',
      'pvc.reference_price_per_roll': '280',
    }

    expect(validatePurchase(values)).toEqual({})
    const input = toPurchaseInput(values).lines[0]!
    expect(input).not.toHaveProperty('product_id')
    expect(input.new_product).toMatchObject({
      sku: 'PVC-NUEVO',
      product_type: 'pvc',
      pvc: { width_m: '1.40', nominal_length_m: '20.00', reference_price_per_roll: '280.00' },
    })
  })
})

describe('resizeRolls', () => {
  const base = [{ ...newRoll(), length: '19,8', cost: '280' }]

  it('adds rolls that copy the last one, with their own identity', () => {
    const rolls = resizeRolls(base, 3)

    expect(rolls).toHaveLength(3)
    expect(rolls.map((roll) => [roll.length, roll.cost])).toEqual([['19,8', '280'], ['19,8', '280'], ['19,8', '280']])
    expect(new Set(rolls.map((roll) => roll.key)).size).toBe(3)
    expect(rolls[0]).toBe(base[0])
  })

  it('removes from the end, keeping the first rolls', () => {
    const three = resizeRolls(base, 3)
    three[1] = { ...three[1]!, length: '20' }

    expect(resizeRolls(three, 2).map((roll) => roll.length)).toEqual(['19,8', '20'])
  })

  it('never goes below one roll nor above the limit', () => {
    expect(resizeRolls(base, 0)).toHaveLength(1)
    expect(resizeRolls(base, 500)).toHaveLength(100)
  })
})

