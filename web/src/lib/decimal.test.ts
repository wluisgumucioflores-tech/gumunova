import { describe, expect, it } from 'vitest'
import { formatDateEs, formatDecimalEs, formatSignedDecimalEs, formatUnitsEs, isValidDecimalInput, toApiDecimal, toInputDecimal } from './decimal'

describe('isValidDecimalInput', () => {
  it.each(['1', '15', '15.5', '15,5', '15.50', '0,05', ' 7 '])('accepts %j', (text) => {
    expect(isValidDecimalInput(text)).toBe(true)
  })

  it.each(['', 'abc', '1.234', '1,234', '-1', '.5', '5.', '1e3', '12345678901', '1.2.3'])('rejects %j', (text) => {
    expect(isValidDecimalInput(text)).toBe(false)
  })
})

describe('toApiDecimal', () => {
  it('normalizes separators and pads to two decimals', () => {
    expect(toApiDecimal('1,4')).toBe('1.40')
    expect(toApiDecimal('15')).toBe('15.00')
    expect(toApiDecimal(' 0,5 ')).toBe('0.50')
    expect(toApiDecimal('15.55')).toBe('15.55')
  })

  it('removes meaningless leading zeros but keeps a single zero', () => {
    expect(toApiDecimal('007,10')).toBe('7.10')
    expect(toApiDecimal('0')).toBe('0.00')
  })

  it('returns null for invalid text', () => {
    expect(toApiDecimal('abc')).toBeNull()
    expect(toApiDecimal('1.234')).toBeNull()
    expect(toApiDecimal('')).toBeNull()
  })
})

describe('formatDecimalEs', () => {
  it('uses dot for thousands and comma for decimals', () => {
    expect(formatDecimalEs('1234.5')).toBe('1.234,50')
    expect(formatDecimalEs('1234567.89')).toBe('1.234.567,89')
    expect(formatDecimalEs('15.50')).toBe('15,50')
    expect(formatDecimalEs('0.05')).toBe('0,05')
    expect(formatDecimalEs('280')).toBe('280,00')
  })
})

describe('toInputDecimal', () => {
  it('shows the Spanish decimal separator', () => {
    expect(toInputDecimal('1.40')).toBe('1,40')
  })
})

describe('formatDateEs', () => {
  it('writes day/month/year without involving time zones', () => {
    expect(formatDateEs('2026-11-10')).toBe('10/11/2026')
    expect(formatDateEs('2026-01-01')).toBe('01/01/2026')
  })

  it('leaves unexpected text untouched', () => {
    expect(formatDateEs('hoy')).toBe('hoy')
  })
})

describe('formatUnitsEs', () => {
  it('drops the decimals and groups thousands', () => {
    expect(formatUnitsEs('10.00')).toBe('10')
    expect(formatUnitsEs('1234.00')).toBe('1.234')
    expect(formatUnitsEs('0')).toBe('0')
  })
})

describe('formatSignedDecimalEs', () => {
  it('always shows the direction of the change', () => {
    expect(formatSignedDecimalEs('19.80')).toBe('+19,80')
    expect(formatSignedDecimalEs('-5.00')).toBe('−5,00')
    expect(formatSignedDecimalEs('1234.5')).toBe('+1.234,50')
  })
})
