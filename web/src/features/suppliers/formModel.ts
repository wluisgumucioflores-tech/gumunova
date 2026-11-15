import type { Supplier, SupplierInput } from '../../api/suppliers'

export type SupplierField = 'name' | 'phone' | 'email' | 'notes'
export type SupplierValues = Record<SupplierField, string>

export const SUPPLIER_FIELDS: SupplierField[] = ['name', 'phone', 'email', 'notes']

/** Codes understood by the Spanish catalog in i18n/es.ts (fieldErrors). */
export type SupplierValidationCode = 'required' | 'too_long' | 'invalid_email'

const MAX_NAME = 120
const MAX_PHONE = 30
const MAX_EMAIL = 254
const MAX_NOTES = 500
const EMAIL_PATTERN = /^[^\s@<>]+@[^\s@<>]+\.[^\s@<>.]+$/

export function emptySupplierValues(): SupplierValues {
  return { name: '', phone: '', email: '', notes: '' }
}

export function valuesFromSupplier(supplier: Supplier): SupplierValues {
  return { name: supplier.name, phone: supplier.phone, email: supplier.email, notes: supplier.notes }
}

/**
 * Quick checks that save a round trip. The backend validates everything again and is the
 * authority; its answers are shown the same way.
 */
export function validateSupplier(values: SupplierValues): Partial<Record<SupplierField, SupplierValidationCode>> {
  const problems: Partial<Record<SupplierField, SupplierValidationCode>> = {}

  const name = values.name.trim()
  if (!name) problems.name = 'required'
  else if (name.length > MAX_NAME) problems.name = 'too_long'

  if (values.phone.trim().length > MAX_PHONE) problems.phone = 'too_long'

  const email = values.email.trim()
  if (email.length > MAX_EMAIL) problems.email = 'too_long'
  else if (email && !EMAIL_PATTERN.test(email)) problems.email = 'invalid_email'

  if (values.notes.trim().length > MAX_NOTES) problems.notes = 'too_long'

  return problems
}

export function toSupplierInput(values: SupplierValues): SupplierInput {
  return {
    name: values.name.trim(),
    phone: values.phone.trim(),
    email: values.email.trim(),
    notes: values.notes.trim(),
  }
}
