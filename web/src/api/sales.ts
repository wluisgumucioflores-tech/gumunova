import { jsonRequest, request, requestWithResponse } from './client'
import type { Page } from './inventory'

// Money and measures are decimal strings such as "15.50".

export type SaleKind = 'roll_meters' | 'full_roll' | 'tablecloth_units'

export interface SaleLine {
  id: string
  number: number
  kind: SaleKind
  product: { id: string; sku: string; name: string }
  roll: { id: string; code: string } | null
  /** Meters, 1 for a whole roll, or units. */
  quantity: string
  unit: 'm' | 'roll' | 'unit'
  /** What was charged per meter, per roll or per unit. */
  unit_price: string
  /** The catalog price when the sale was made. */
  reference_price: string
  line_total: string
  cost_of_sale: string
  gross_profit: string
}

export interface Sale {
  id: string
  sale_date: string
  notes: string
  status: 'confirmed'
  total_amount: string
  total_cost: string
  gross_profit: string
  lines: SaleLine[]
  created_at: string
}

export interface SaleSummary {
  id: string
  sale_date: string
  total_amount: string
  total_cost: string
  gross_profit: string
  line_count: number
  created_at: string
}

export interface SaleLineInput {
  kind: SaleKind
  roll_id?: string
  product_id?: string
  meters?: string
  quantity?: number
  unit_price: string
}

export interface SaleInput {
  sale_date: string
  notes: string
  lines: SaleLineInput[]
}

export interface SaleFilters {
  from?: string
  to?: string
  query?: string
  limit?: number
  offset?: number
}

export function listSales(filters: SaleFilters = {}, signal?: AbortSignal): Promise<Page<SaleSummary>> {
  const params = new URLSearchParams()
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  if (filters.query?.trim()) params.set('q', filters.query.trim())
  if (filters.limit !== undefined) params.set('limit', String(filters.limit))
  if (filters.offset !== undefined) params.set('offset', String(filters.offset))
  const query = params.toString()
  return request<Page<SaleSummary>>(`/sales${query ? `?${query}` : ''}`, { signal })
}

export function getSale(id: string, signal?: AbortSignal): Promise<Sale> {
  return request<Sale>(`/sales/${encodeURIComponent(id)}`, { signal })
}

export interface RegisterSaleResult {
  sale: Sale
  /** True when the server had already registered this same request and returned that sale. */
  replayed: boolean
}

/**
 * Registers a sale. The idempotency key identifies this attempt: sending the same key again
 * (a retry or a double tap) never sells twice. A sale is only confirmed once this resolves.
 */
export async function registerSale(input: SaleInput, idempotencyKey: string): Promise<RegisterSaleResult> {
  const { data, response } = await requestWithResponse<Sale>(
    '/sales',
    jsonRequest('POST', input, { 'Idempotency-Key': idempotencyKey }),
  )
  return { sale: data, replayed: response.headers.get('Idempotent-Replayed') === 'true' }
}
