import { describe, expect, it } from 'vitest'
import { supplier } from '../../test/fixtures'
import { emptySupplierValues, toSupplierInput, validateSupplier, valuesFromSupplier } from './formModel'

describe('validateSupplier', () => {
  it('accepts a name alone', () => {
    expect(validateSupplier({ ...emptySupplierValues(), name: 'Proveedor' })).toEqual({})
  })

  it('requires the name', () => {
    expect(validateSupplier(emptySupplierValues())).toEqual({ name: 'required' })
    expect(validateSupplier({ ...emptySupplierValues(), name: '   ' })).toEqual({ name: 'required' })
  })

  it.each(['ventas@sur.com', 'a@b.co', 'ana.perez+pvc@correo.net'])('accepts the email %s', (email) => {
    expect(validateSupplier({ ...emptySupplierValues(), name: 'P', email })).toEqual({})
  })

  it.each(['sin-arroba', '@dominio.com', 'a@', 'a@dominio', 'a@dominio.', 'a b@x.com', 'Ana <ana@x.com>'])(
    'rejects the email %s',
    (email) => {
      expect(validateSupplier({ ...emptySupplierValues(), name: 'P', email })).toEqual({ email: 'invalid_email' })
    },
  )

  it('limits the length of every field', () => {
    const problems = validateSupplier({
      name: 'n'.repeat(121),
      phone: '1'.repeat(31),
      email: 'a'.repeat(250) + '@x.com',
      notes: 'x'.repeat(501),
    })
    expect(problems).toEqual({ name: 'too_long', phone: 'too_long', email: 'too_long', notes: 'too_long' })
  })
})

describe('toSupplierInput', () => {
  it('trims every field', () => {
    expect(toSupplierInput({ name: ' P ', phone: ' 7 ', email: ' a@b.co ', notes: ' n ' })).toEqual({
      name: 'P',
      phone: '7',
      email: 'a@b.co',
      notes: 'n',
    })
  })

  it('round-trips a supplier', () => {
    const original = supplier()
    expect(toSupplierInput(valuesFromSupplier(original))).toEqual({
      name: original.name,
      phone: original.phone,
      email: original.email,
      notes: original.notes,
    })
  })
})
