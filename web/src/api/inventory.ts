import { request } from './client'
import type { ProductType } from './products'

// Money and measures are decimal strings such as "15.50". Cost per meter keeps four decimals and,
// like the remaining value, is for reading only: the backend never uses it to calculate.

export interface StockItem {
  product_id: string
  sku: string
  name: string
  product_type: ProductType
  is_active: boolean
  /** Meters (PVC) or units (tablecloths) available. */
  quantity: string
  unit: 'm' | 'unit'
  /** Minimum stock in the same unit; zero means no alert. */
  minimum: string
  is_low: boolean
  /** Rolls with meters left. null for tablecloths. */
  roll_count: number | null
  stock_value: string
  /** Weighted average cost per unit. null for PVC (each roll has its own) and for empty stock. */
  average_cost: string | null
}

export interface Roll {
  id: string
  code: string
  /** Unique code printed on the roll's label (QR or barcode); different from `code`. */
  label_code: string
  product: { id: string; sku: string; name: string }
  purchase_id: string
  received_at: string
  initial_length_m: string
  available_length_m: string
  acquisition_cost: string
  additional_cost: string
  attributable_cost: string
  cost_per_meter: string
  remaining_value: string
  /** True while the roll has had no cut or other outflow. */
  is_intact: boolean
}

export type MovementType = 'purchase_in' | 'purchase_reversal' | 'sale_out'

export interface Movement {
  id: string
  created_at: string
  product: { id: string; sku: string; name: string }
  roll: { id: string; code: string } | null
  movement_type: MovementType
  /** Positive adds stock, negative takes it out. */
  quantity_delta: string
  unit: 'm' | 'unit'
  reference_type: string
  reference_id: string
  reason: string
}

export interface Page<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

export interface StockFilters {
  type?: ProductType
  onlyLow?: boolean
  query?: string
  limit?: number
  offset?: number
}

export type RollStatus = 'available' | 'depleted' | 'all'

export interface RollFilters {
  productId?: string
  status?: RollStatus
  query?: string
  limit?: number
  offset?: number
}

export interface MovementFilters {
  productId?: string
  rollId?: string
  type?: MovementType
  limit?: number
  offset?: number
}

function withQuery(path: string, params: URLSearchParams): string {
  const query = params.toString()
  return query ? `${path}?${query}` : path
}

function paging(params: URLSearchParams, limit?: number, offset?: number) {
  if (limit !== undefined) params.set('limit', String(limit))
  if (offset !== undefined) params.set('offset', String(offset))
}

export function listStock(filters: StockFilters = {}, signal?: AbortSignal): Promise<Page<StockItem>> {
  const params = new URLSearchParams()
  if (filters.type) params.set('type', filters.type)
  if (filters.onlyLow) params.set('low', 'true')
  if (filters.query?.trim()) params.set('q', filters.query.trim())
  paging(params, filters.limit, filters.offset)
  return request<Page<StockItem>>(withQuery('/inventory/stock', params), { signal })
}

export function listRolls(filters: RollFilters = {}, signal?: AbortSignal): Promise<Page<Roll>> {
  const params = new URLSearchParams()
  if (filters.productId) params.set('product_id', filters.productId)
  if (filters.status) params.set('status', filters.status)
  if (filters.query?.trim()) params.set('q', filters.query.trim())
  paging(params, filters.limit, filters.offset)
  return request<Page<Roll>>(withQuery('/inventory/rolls', params), { signal })
}

export function listMovements(filters: MovementFilters = {}, signal?: AbortSignal): Promise<Page<Movement>> {
  const params = new URLSearchParams()
  if (filters.productId) params.set('product_id', filters.productId)
  if (filters.rollId) params.set('roll_id', filters.rollId)
  if (filters.type) params.set('type', filters.type)
  paging(params, filters.limit, filters.offset)
  return request<Page<Movement>>(withQuery('/inventory/movements', params), { signal })
}

/** Gives the roll a new label code: labels printed with the old one stop identifying it. No stock changes. */
export function resetRollLabelCode(id: string): Promise<Roll> {
  return request<Roll>(`/inventory/rolls/${encodeURIComponent(id)}/label-code/reset`, { method: 'POST' })
}
