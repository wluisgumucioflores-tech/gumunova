import { jsonRequest, request } from './client'

export interface Supplier {
  id: string
  name: string
  phone: string
  email: string
  notes: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface SupplierList {
  items: Supplier[]
  total: number
  limit: number
  offset: number
}

export interface SupplierInput {
  name: string
  phone: string
  email: string
  notes: string
}

export interface SupplierFilters {
  isActive?: boolean
  query?: string
  limit?: number
  offset?: number
}

export function listSuppliers(filters: SupplierFilters = {}, signal?: AbortSignal): Promise<SupplierList> {
  const params = new URLSearchParams()
  if (filters.isActive !== undefined) params.set('is_active', String(filters.isActive))
  if (filters.query?.trim()) params.set('q', filters.query.trim())
  if (filters.limit !== undefined) params.set('limit', String(filters.limit))
  if (filters.offset !== undefined) params.set('offset', String(filters.offset))
  const query = params.toString()
  return request<SupplierList>(`/suppliers${query ? `?${query}` : ''}`, { signal })
}

export function getSupplier(id: string, signal?: AbortSignal): Promise<Supplier> {
  return request<Supplier>(`/suppliers/${encodeURIComponent(id)}`, { signal })
}

export function createSupplier(input: SupplierInput): Promise<Supplier> {
  return request<Supplier>('/suppliers', jsonRequest('POST', input))
}

export function updateSupplier(id: string, input: SupplierInput): Promise<Supplier> {
  return request<Supplier>(`/suppliers/${encodeURIComponent(id)}`, jsonRequest('PUT', input))
}

export function activateSupplier(id: string): Promise<Supplier> {
  return request<Supplier>(`/suppliers/${encodeURIComponent(id)}/activate`, { method: 'POST' })
}

export function deactivateSupplier(id: string): Promise<Supplier> {
  return request<Supplier>(`/suppliers/${encodeURIComponent(id)}/deactivate`, { method: 'POST' })
}
