import { toApiDecimal } from './decimal'

// Exact money arithmetic for on-screen estimates. Amounts are whole counts of hundredths held in
// a BigInt, so sums never go through floating point. The server is the authority on totals.

/** "12,5" -> 1250n. Returns null when the text is not a valid amount. */
export function toHundredths(text: string): bigint | null {
  const apiDecimal = toApiDecimal(text)
  return apiDecimal === null ? null : BigInt(apiDecimal.replace('.', ''))
}

/** 1250n -> "12.50". */
export function hundredthsToApi(value: bigint): string {
  const negative = value < 0n
  const digits = (negative ? -value : value).toString().padStart(3, '0')
  return `${negative ? '-' : ''}${digits.slice(0, -2)}.${digits.slice(-2)}`
}

/**
 * price × quantity, both in hundredths, rounded half up to the hundredth: 1550n × 125n -> 1938n.
 * The same rule the server applies, so the on-screen total matches the one it confirms.
 */
export function multiplyHundredths(price: bigint, quantity: bigint): bigint {
  return (price * quantity * 2n + 100n) / 200n
}

