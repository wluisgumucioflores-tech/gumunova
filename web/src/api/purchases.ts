import { jsonRequest, request, requestWithResponse } from './client'
import type { ProductInput } from './products'

export interface PurchaseRoll {
  id: string
  roll_code: string
  length_m: string
  acquisition_cost: string
  additional_cost: string
  attributable_cost: string
}

export interface PurchaseLine {
  id: string
  line_number: number
  product: { id: string; sku: string; name: string; product_type: 'pvc' | 'tablecloth' }
  quantity: number
  unit_cost: string | null
  additional_cost: string
  line_total: string
  rolls: PurchaseRoll[]
}

// Money and measures are decimal strings such as "15.50".
export interface Purchase {
  id: string
  /** null when the purchase was registered without a supplier. */
  supplier: { id: string; name: string } | null
  purchase_date: string
  document_reference: string
  notes: string
  additional_costs: string
  total_amount: string
  status: 'confirmed'
  /** Starts at 1 and grows each time the items are replaced. */
  revision: number
  /** False once stock from this purchase was sold: only its data can still be corrected. */
  items_editable: boolean
  lines: PurchaseLine[]
  created_at: string
}

export interface PurchaseSummary {
  id: string
  /** null when the purchase was registered without a supplier. */
  supplier: { id: string; name: string } | null
  purchase_date: string
  document_reference: string
  total_amount: string
  line_count: number
  created_at: string
}

export interface PurchaseList {
  items: PurchaseSummary[]
  total: number
  limit: number
  offset: number
}

/** A line names an existing product, or describes a new one that is created with the purchase. */
interface LineProductInput {
  product_id?: string
  new_product?: ProductInput
}

export interface PvcLineInput extends LineProductInput {
  rolls: { length_m: string; cost: string }[]
}

export interface TableclothLineInput extends LineProductInput {
  quantity: number
  unit_cost: string
}

export interface PurchaseInput {
  /** Empty when the purchase has no supplier. */
  supplier_id: string
  purchase_date: string
  document_reference: string
  notes: string
  additional_costs: string
  lines: (PvcLineInput | TableclothLineInput)[]
}

export interface PurchaseFilters {
  supplierId?: string
  from?: string
  to?: string
  query?: string
  limit?: number
  offset?: number
}

export function listPurchases(filters: PurchaseFilters = {}, signal?: AbortSignal): Promise<PurchaseList> {
  const params = new URLSearchParams()
  if (filters.supplierId) params.set('supplier_id', filters.supplierId)
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  if (filters.query?.trim()) params.set('q', filters.query.trim())
  if (filters.limit !== undefined) params.set('limit', String(filters.limit))
  if (filters.offset !== undefined) params.set('offset', String(filters.offset))
  const query = params.toString()
  return request<PurchaseList>(`/purchases${query ? `?${query}` : ''}`, { signal })
}

export function getPurchase(id: string, signal?: AbortSignal): Promise<Purchase> {
  return request<Purchase>(`/purchases/${encodeURIComponent(id)}`, { signal })
}

export interface RegisterResult {
  purchase: Purchase
  /** True when the server had already registered this same request and returned that purchase. */
  replayed: boolean
}

/**
 * Registers a purchase. The idempotency key identifies this attempt: sending the same key again
 * (a retry or a double tap) never creates a second purchase.
 */
export async function registerPurchase(input: PurchaseInput, idempotencyKey: string): Promise<RegisterResult> {
  const { data, response } = await requestWithResponse<Purchase>(
    '/purchases',
    jsonRequest('POST', input, { 'Idempotency-Key': idempotencyKey }),
  )
  return { purchase: data, replayed: response.headers.get('Idempotent-Replayed') === 'true' }
}

/** Corrects a purchase. The items change only while nothing from it was sold (otherwise 409). */
export function updatePurchase(id: string, input: PurchaseInput): Promise<Purchase> {
  return request<Purchase>(`/purchases/${encodeURIComponent(id)}`, jsonRequest('PUT', input))
}
