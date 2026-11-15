import { request } from './client'
import type { Product } from './products'
import type { Roll } from './inventory'

/** What a scanned label identifies: a product, or one physical roll together with its product. */
export interface LabelLookup {
  kind: 'product' | 'roll'
  /** Holds the reference prices, also for a roll. */
  product: Product
  roll?: Roll
}

/** Reads the code of a printed label. A 404 `label_not_found` means the code is unknown or outdated. */
export function lookupLabel(code: string, signal?: AbortSignal): Promise<LabelLookup> {
  return request<LabelLookup>(`/labels/${encodeURIComponent(code.trim())}`, { signal })
}
