// Money and measures are exact decimals handled as strings ("15.50"). They are never turned
// into `number`, so no value is ever rounded by floating point arithmetic.

const DECIMAL_INPUT = /^\d{1,10}(?:[.,]\d{1,2})?$/

/** True when the text is a non-negative decimal with at most two fractional digits. */
export function isValidDecimalInput(text: string): boolean {
  return DECIMAL_INPUT.test(text.trim())
}

/**
 * Converts what a person types ("1,4", " 15 ") into the canonical API form ("1.40", "15.00").
 * Returns null when the text is not a valid decimal.
 */
export function toApiDecimal(text: string): string | null {
  const trimmed = text.trim()
  if (!DECIMAL_INPUT.test(trimmed)) {
    return null
  }
  const [whole = '0', fraction = ''] = trimmed.replace(',', '.').split('.')
  return `${whole.replace(/^0+(?=\d)/, '')}.${fraction.padEnd(2, '0')}`
}

/** Formats an API decimal for display in Spanish: "1234.5" -> "1.234,50". */
export function formatDecimalEs(apiDecimal: string): string {
  const [whole = '0', fraction = '00'] = apiDecimal.split('.')
  const grouped = whole.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return `${grouped},${fraction.padEnd(2, '0')}`
}

/** Converts an API decimal into the text shown inside an input: "1.40" -> "1,40". */
export function toInputDecimal(apiDecimal: string): string {
  return apiDecimal.replace('.', ',')
}

/** "2026-11-10" -> "10/11/2026". Works on the text, so no time zone can shift the day. */
export function formatDateEs(isoDate: string): string {
  const [year, month, day] = isoDate.split('-')
  return year && month && day ? `${day}/${month}/${year}` : isoDate
}

/** Whole units without decimals: "1234.00" -> "1.234". */
export function formatUnitsEs(apiDecimal: string): string {
  const [whole = '0'] = apiDecimal.split('.')
  return whole.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
}

/** A change in stock with its sign: "19.80" -> "+19,80", "-5.00" -> "−5,00". */
export function formatSignedDecimalEs(apiDecimal: string): string {
  const negative = apiDecimal.startsWith('-')
  const magnitude = formatDecimalEs(negative ? apiDecimal.slice(1) : apiDecimal)
  return `${negative ? '−' : '+'}${magnitude}`
}

