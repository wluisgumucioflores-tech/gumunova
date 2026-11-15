import type { Product, ProductList } from '../api/products'
import type { Purchase, PurchaseList, PurchaseSummary } from '../api/purchases'
import type { Movement, Page, Roll, StockItem } from '../api/inventory'
import type { Sale, SaleSummary } from '../api/sales'
import type { Supplier, SupplierList } from '../api/suppliers'

export function pvcProduct(overrides: Partial<Product> = {}): Product {
  return {
    id: 'pvc-1',
    sku: 'PVC-FLORAL-20',
    name: 'PVC floral 20 m',
    product_type: 'pvc',
    is_active: true,
    image_url: null,
    label_code: 'P-AAAAAAAAAAAA',
    pvc: {
      material: 'PVC',
      design: 'Floral',
      color: 'Rojo',
      texture: 'Lisa',
      width_m: '1.40',
      nominal_length_m: '20.00',
      thickness_mm: null,
      reference_price_per_meter: '15.50',
      reference_price_per_roll: '1280.00',
      minimum_stock_m: '0.00',
    },
    created_at: '2026-11-11T12:00:00Z',
    updated_at: '2026-11-11T12:00:00Z',
    ...overrides,
  }
}

export function tableclothProduct(overrides: Partial<Product> = {}): Product {
  return {
    id: 'cloth-1',
    sku: 'MAN-ROJO-01',
    name: 'Mantel rojo',
    product_type: 'tablecloth',
    is_active: true,
    image_url: null,
    label_code: 'P-BBBBBBBBBBBB',
    tablecloth: {
      design: 'Liso',
      color: 'Rojo',
      dimensions: '1,40 x 2,00 m',
      reference_cost: null,
      unit_price: '60.00',
      minimum_units: 0,
    },
    created_at: '2026-11-11T12:00:00Z',
    updated_at: '2026-11-11T12:00:00Z',
    ...overrides,
  }
}

export function listOf(items: Product[], total = items.length): ProductList {
  return { items, total, limit: 25, offset: 0 }
}

export function supplier(overrides: Partial<Supplier> = {}): Supplier {
  return {
    id: 'sup-1',
    name: 'Plásticos del Sur',
    phone: '+591 700 00000',
    email: 'ventas@sur.com',
    notes: 'Entrega en 3 días',
    is_active: true,
    created_at: '2026-11-11T12:00:00Z',
    updated_at: '2026-11-11T12:00:00Z',
    ...overrides,
  }
}

export function supplierListOf(items: Supplier[], total = items.length): SupplierList {
  return { items, total, limit: 25, offset: 0 }
}

export function purchase(overrides: Partial<Purchase> = {}): Purchase {
  return {
    id: 'pur-1',
    supplier: { id: 'sup-1', name: 'Plásticos del Sur' },
    purchase_date: '2026-11-10',
    document_reference: 'FAC-001',
    notes: 'Entrega parcial',
    additional_costs: '30.00',
    total_amount: '910.00',
    status: 'confirmed',
    revision: 1,
    items_editable: true,
    lines: [
      {
        id: 'line-1',
        line_number: 1,
        product: { id: 'pvc-1', sku: 'PVC-FLORAL-20', name: 'PVC floral 20 m', product_type: 'pvc' },
        quantity: 2,
        unit_cost: null,
        additional_cost: '19.77',
        line_total: '599.77',
        rolls: [
          {
            id: 'roll-1',
            roll_code: 'ROL-000001',
            length_m: '19.80',
            acquisition_cost: '280.00',
            additional_cost: '9.54',
            attributable_cost: '289.54',
          },
          {
            id: 'roll-2',
            roll_code: 'ROL-000002',
            length_m: '20.00',
            acquisition_cost: '300.00',
            additional_cost: '10.23',
            attributable_cost: '310.23',
          },
        ],
      },
      {
        id: 'line-2',
        line_number: 2,
        product: { id: 'cloth-1', sku: 'MAN-ROJO-01', name: 'Mantel rojo', product_type: 'tablecloth' },
        quantity: 10,
        unit_cost: '30.00',
        additional_cost: '10.23',
        line_total: '310.23',
        rolls: [],
      },
    ],
    created_at: '2026-11-11T12:00:00Z',
    ...overrides,
  }
}

export function purchaseSummary(overrides: Partial<PurchaseSummary> = {}): PurchaseSummary {
  return {
    id: 'pur-1',
    supplier: { id: 'sup-1', name: 'Plásticos del Sur' },
    purchase_date: '2026-11-10',
    document_reference: 'FAC-001',
    total_amount: '1234.50',
    line_count: 2,
    created_at: '2026-11-11T12:00:00Z',
    ...overrides,
  }
}

export function purchaseListOf(items: PurchaseSummary[], total = items.length): PurchaseList {
  return { items, total, limit: 25, offset: 0 }
}

export function pageOf<T>(items: T[], total = items.length): Page<T> {
  return { items, total, limit: 25, offset: 0 }
}

export function stockItem(overrides: Partial<StockItem> = {}): StockItem {
  return {
    product_id: 'pvc-1',
    sku: 'PVC-FLORAL-20',
    name: 'PVC floral 20 m',
    product_type: 'pvc',
    is_active: true,
    quantity: '39.80',
    unit: 'm',
    minimum: '50.00',
    is_low: true,
    roll_count: 2,
    stock_value: '599.77',
    average_cost: null,
    ...overrides,
  }
}

export function clothStockItem(overrides: Partial<StockItem> = {}): StockItem {
  return stockItem({
    product_id: 'cloth-1',
    sku: 'MAN-ROJO-01',
    name: 'Mantel rojo',
    product_type: 'tablecloth',
    quantity: '1234.00',
    unit: 'unit',
    minimum: '0',
    is_low: false,
    roll_count: null,
    stock_value: '310.23',
    average_cost: '31.0230',
    ...overrides,
  })
}

export function roll(overrides: Partial<Roll> = {}): Roll {
  return {
    id: 'roll-1',
    code: 'ROL-000001',
    label_code: 'R-111111111111',
    product: { id: 'pvc-1', sku: 'PVC-FLORAL-20', name: 'PVC floral 20 m' },
    purchase_id: 'pur-1',
    received_at: '2026-11-10',
    initial_length_m: '19.80',
    available_length_m: '19.80',
    acquisition_cost: '280.00',
    additional_cost: '9.54',
    attributable_cost: '289.54',
    cost_per_meter: '14.6232',
    remaining_value: '289.54',
    is_intact: true,
    ...overrides,
  }
}

export function movement(overrides: Partial<Movement> = {}): Movement {
  return {
    id: 'mov-1',
    created_at: '2026-11-10T15:05:00Z',
    product: { id: 'pvc-1', sku: 'PVC-FLORAL-20', name: 'PVC floral 20 m' },
    roll: { id: 'roll-1', code: 'ROL-000001' },
    movement_type: 'purchase_in',
    quantity_delta: '19.80',
    unit: 'm',
    reference_type: 'purchase',
    reference_id: 'pur-1',
    reason: '',
    ...overrides,
  }
}

export function sale(overrides: Partial<Sale> = {}): Sale {
  return {
    id: 'sale-1',
    sale_date: '2026-11-10',
    notes: 'Cliente de paso',
    status: 'confirmed',
    total_amount: '305.00',
    total_cost: '161.65',
    gross_profit: '143.35',
    lines: [
      {
        id: 'line-1',
        number: 1,
        kind: 'roll_meters',
        product: { id: 'pvc-1', sku: 'PVC-FLORAL-20', name: 'PVC floral 20 m' },
        roll: { id: 'roll-1', code: 'ROL-000001' },
        quantity: '2.50',
        unit: 'm',
        unit_price: '18.00',
        reference_price: '15.50',
        line_total: '45.00',
        cost_of_sale: '36.56',
        gross_profit: '8.44',
      },
      {
        id: 'line-2',
        number: 2,
        kind: 'tablecloth_units',
        product: { id: 'cloth-1', sku: 'MAN-ROJO-01', name: 'Mantel rojo' },
        roll: null,
        quantity: '4.00',
        unit: 'unit',
        unit_price: '65.00',
        reference_price: '60.00',
        line_total: '260.00',
        cost_of_sale: '125.09',
        gross_profit: '134.91',
      },
    ],
    created_at: '2026-11-11T12:00:00Z',
    ...overrides,
  }
}

export function saleSummary(overrides: Partial<SaleSummary> = {}): SaleSummary {
  return {
    id: 'sale-1',
    sale_date: '2026-11-10',
    total_amount: '305.00',
    total_cost: '161.65',
    gross_profit: '143.35',
    line_count: 2,
    created_at: '2026-11-11T12:00:00Z',
    ...overrides,
  }
}

