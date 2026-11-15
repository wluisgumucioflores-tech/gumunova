import { describe, expect, it } from 'vitest'
import { hundredthsToApi, multiplyHundredths, toHundredths } from './money'

describe('toHundredths', () => {
  it('reads what a person types, with a comma or a dot', () => {
    expect(toHundredths('12,5')).toBe(1250n)
    expect(toHundredths('12.50')).toBe(1250n)
    expect(toHundredths('0,05')).toBe(5n)
    expect(toHundredths(' 7 ')).toBe(700n)
  })

  it('returns null for text that is not an amount', () => {
    expect(toHundredths('')).toBeNull()
    expect(toHundredths('abc')).toBeNull()
    expect(toHundredths('1,234')).toBeNull()
    expect(toHundredths('-1')).toBeNull()
  })
})

describe('hundredthsToApi', () => {
  it('writes two decimals', () => {
    expect(hundredthsToApi(1250n)).toBe('12.50')
    expect(hundredthsToApi(5n)).toBe('0.05')
    expect(hundredthsToApi(0n)).toBe('0.00')
    expect(hundredthsToApi(-150n)).toBe('-1.50')
  })

  it('is exact where floating point is not', () => {
    // 0.1 + 0.2 === 0.30000000000000004 with numbers.
    const sum = toHundredths('0,10')! + toHundredths('0,20')!
    expect(hundredthsToApi(sum)).toBe('0.30')
  })

  it('handles amounts beyond the safe integer range of numbers', () => {
    const huge = toHundredths('9999999999,99')! * 100_000n
    expect(huge > BigInt(Number.MAX_SAFE_INTEGER)).toBe(true)
    expect(hundredthsToApi(huge)).toBe('999999999999000.00')
  })
})

describe('multiplyHundredths', () => {
  it('rounds half up to the hundredth, like the server', () => {
    expect(multiplyHundredths(1550n, 125n)).toBe(1938n) // 15,50 x 1,25 = 19,375 -> 19,38
    expect(multiplyHundredths(1n, 50n)).toBe(1n) // 0,01 x 0,50 = 0,005 -> 0,01
    expect(multiplyHundredths(1n, 49n)).toBe(0n)
    expect(multiplyHundredths(28000n, 100n)).toBe(28000n)
    expect(multiplyHundredths(6000n, 1000n)).toBe(60000n)
  })

  it('stays exact with large amounts', () => {
    expect(multiplyHundredths(99_999_999_999n, 10_000_000n)).toBe(9_999_999_999_900_000n)
  })
})

