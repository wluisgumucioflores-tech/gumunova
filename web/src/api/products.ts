import { jsonRequest, request } from './client'

export type ProductType = 'pvc' | 'tablecloth'

// Money and measures are decimal strings such as "15.50".
export interface PvcDetails {
  material: string
  design: string
  color: string
  texture: string
  width_m: string
  nominal_length_m: string
  thickness_mm: string | null
  reference_price_per_meter: string
  reference_price_per_roll: string
  /** Meters across all rolls at or below which stock is flagged low; "0.00" disables the alert. */
  minimum_stock_m: string
}

export interface TableclothDetails {
  design: string
  color: string
  dimensions: string
  reference_cost: string | null
  unit_price: string
  /** Units at or below which stock is flagged low; 0 disables the alert. */
  minimum_units: number
}

export interface Product {
  id: string
  sku: string
  name: string
  product_type: ProductType
  is_active: boolean
  image_url: string | null
  /** Unique code printed on the product's labels (QR or barcode). */
  label_code: string
  pvc?: PvcDetails
  tablecloth?: TableclothDetails
  created_at: string
  updated_at: string
}

export interface ProductList {
  items: Product[]
  total: number
  limit: number
  offset: number
}

export interface ProductInput {
  sku: string
  name: string
  product_type: ProductType
  pvc?: PvcDetails
  tablecloth?: TableclothDetails
}

export interface ProductFilters {
  type?: ProductType
  isActive?: boolean
  query?: string
  limit?: number
  offset?: number
}

export function listProducts(filters: ProductFilters = {}, signal?: AbortSignal): Promise<ProductList> {
  const params = new URLSearchParams()
  if (filters.type) params.set('type', filters.type)
  if (filters.isActive !== undefined) params.set('is_active', String(filters.isActive))
  if (filters.query?.trim()) params.set('q', filters.query.trim())
  if (filters.limit !== undefined) params.set('limit', String(filters.limit))
  if (filters.offset !== undefined) params.set('offset', String(filters.offset))
  const query = params.toString()
  return request<ProductList>(`/products${query ? `?${query}` : ''}`, { signal })
}

export function getProduct(id: string, signal?: AbortSignal): Promise<Product> {
  return request<Product>(`/products/${encodeURIComponent(id)}`, { signal })
}

export function createProduct(input: ProductInput): Promise<Product> {
  return request<Product>('/products', jsonRequest('POST', input))
}

export function updateProduct(id: string, input: ProductInput): Promise<Product> {
  return request<Product>(`/products/${encodeURIComponent(id)}`, jsonRequest('PUT', input))
}

export function activateProduct(id: string): Promise<Product> {
  return request<Product>(`/products/${encodeURIComponent(id)}/activate`, { method: 'POST' })
}

export function deactivateProduct(id: string): Promise<Product> {
  return request<Product>(`/products/${encodeURIComponent(id)}/deactivate`, { method: 'POST' })
}

export function uploadProductImage(id: string, file: File): Promise<Product> {
  const body = new FormData()
  body.append('image', file)
  return request<Product>(`/products/${encodeURIComponent(id)}/image`, { method: 'PUT', body })
}

export function removeProductImage(id: string): Promise<Product> {
  return request<Product>(`/products/${encodeURIComponent(id)}/image`, { method: 'DELETE' })
}

/** Gives the product a new label code: labels printed with the old one stop identifying it. */
export function resetProductLabelCode(id: string): Promise<Product> {
  return request<Product>(`/products/${encodeURIComponent(id)}/label-code/reset`, { method: 'POST' })
}
