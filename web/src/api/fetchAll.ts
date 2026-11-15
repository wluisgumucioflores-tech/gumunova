import { listRolls, listStock, type Roll, type RollStatus, type StockItem } from './inventory'
import { listProducts, type Product, type ProductType } from './products'
import { listSuppliers, type Supplier } from './suppliers'

const PAGE_SIZE = 200

/** Reads every page of a list, for option menus that need the whole set. */
async function fetchAll<T>(
  fetchPage: (offset: number) => Promise<{ items: T[]; total: number }>,
  signal?: AbortSignal,
): Promise<T[]> {
  const items: T[] = []
  for (;;) {
    if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    const page = await fetchPage(items.length)
    items.push(...page.items)
    if (page.items.length === 0 || items.length >= page.total) return items
  }
}

export function listAllActiveSuppliers(signal?: AbortSignal): Promise<Supplier[]> {
  return fetchAll((offset) => listSuppliers({ isActive: true, limit: PAGE_SIZE, offset }, signal), signal)
}

export function listAllActiveProducts(type: ProductType, signal?: AbortSignal): Promise<Product[]> {
  return fetchAll((offset) => listProducts({ type, isActive: true, limit: PAGE_SIZE, offset }, signal), signal)
}

/** Every product of any type, optionally only the active ones. */
export function listAllProducts(isActive?: boolean, signal?: AbortSignal): Promise<Product[]> {
  return fetchAll((offset) => listProducts({ isActive, limit: PAGE_SIZE, offset }, signal), signal)
}

/** Every supplier, active or not: historical purchases can belong to a deactivated one. */
export function listAllSuppliers(signal?: AbortSignal): Promise<Supplier[]> {
  return fetchAll((offset) => listSuppliers({ limit: PAGE_SIZE, offset }, signal), signal)
}

/** Every roll that still has meters, for choosing what to sell. */
export function listAllAvailableRolls(signal?: AbortSignal): Promise<Roll[]> {
  return listAllRolls('available', signal)
}

/** Every roll with the given status (`all` includes the depleted ones). */
export function listAllRolls(status: RollStatus, signal?: AbortSignal): Promise<Roll[]> {
  return fetchAll((offset) => listRolls({ status, limit: PAGE_SIZE, offset }, signal), signal)
}

/** Stock of every tablecloth variant, to know how many units can be sold. */
export function listAllTableclothStock(signal?: AbortSignal): Promise<StockItem[]> {
  return fetchAll((offset) => listStock({ type: 'tablecloth', limit: PAGE_SIZE, offset }, signal), signal)
}

