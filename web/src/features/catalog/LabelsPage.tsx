import { listAllProducts } from '../../api/fetchAll'
import type { Product } from '../../api/products'
import { es } from '../../i18n/es'
import { LabelsScreen, type LabelEntry } from '../labels/LabelsScreen'

const loadProducts = (everything: boolean, signal: AbortSignal) => listAllProducts(everything ? true : undefined, signal)
const entryOf = (product: Product): LabelEntry => ({ code: product.label_code, primary: product.sku, secondary: product.name })

/** Labels of products: each carries the product's own label code, not its SKU. */
export function LabelsPage() {
  return <LabelsScreen<Product> strings={es.catalog.labels} backTo="/catalogo" loadItems={loadProducts} entryOf={entryOf} />
}
