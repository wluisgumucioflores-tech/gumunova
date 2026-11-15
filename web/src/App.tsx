import { Link, Navigate, Route, Routes } from 'react-router'
import { AppLayout } from './components/AppLayout'
import { LabelsPage } from './features/catalog/LabelsPage'
import { ProductCreatePage } from './features/catalog/ProductCreatePage'
import { ProductEditPage } from './features/catalog/ProductEditPage'
import { ProductListPage } from './features/catalog/ProductListPage'
import { PurchaseCreatePage } from './features/purchases/PurchaseCreatePage'
import { InventoryLayout } from './features/inventory/InventoryLayout'
import { MovementsPage } from './features/inventory/MovementsPage'
import { RollLabelsPage } from './features/inventory/RollLabelsPage'
import { RollsPage } from './features/inventory/RollsPage'
import { StockPage } from './features/inventory/StockPage'
import { PurchaseEditPage } from './features/purchases/PurchaseEditPage'
import { PurchaseDetailPage } from './features/purchases/PurchaseDetailPage'
import { PurchaseListPage } from './features/purchases/PurchaseListPage'
import { ScanPage } from './features/scan/ScanPage'
import { SaleCreatePage } from './features/sales/SaleCreatePage'
import { SaleDetailPage } from './features/sales/SaleDetailPage'
import { SaleListPage } from './features/sales/SaleListPage'
import { SupplierCreatePage } from './features/suppliers/SupplierCreatePage'
import { SupplierEditPage } from './features/suppliers/SupplierEditPage'
import { SupplierListPage } from './features/suppliers/SupplierListPage'
import { es } from './i18n/es'

export function App() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route index element={<Navigate to="/catalogo" replace />} />
        <Route path="catalogo" element={<ProductListPage />} />
        <Route path="catalogo/nuevo" element={<ProductCreatePage />} />
        <Route path="catalogo/etiquetas" element={<LabelsPage />} />
        <Route path="catalogo/:id/editar" element={<ProductEditPage />} />
        <Route path="proveedores" element={<SupplierListPage />} />
        <Route path="proveedores/nuevo" element={<SupplierCreatePage />} />
        <Route path="proveedores/:id/editar" element={<SupplierEditPage />} />
        <Route path="compras" element={<PurchaseListPage />} />
        <Route path="compras/nueva" element={<PurchaseCreatePage />} />
        <Route path="compras/:id" element={<PurchaseDetailPage />} />
        <Route path="compras/:id/editar" element={<PurchaseEditPage />} />
        <Route path="ventas" element={<SaleListPage />} />
        <Route path="ventas/nueva" element={<SaleCreatePage />} />
        <Route path="ventas/:id" element={<SaleDetailPage />} />
        <Route path="escanear" element={<ScanPage />} />
        <Route path="producto/:code" element={<ScanPage />} />
        <Route path="inventario" element={<InventoryLayout />}>
          <Route index element={<StockPage />} />
          <Route path="rollos" element={<RollsPage />} />
          <Route path="rollos/etiquetas" element={<RollLabelsPage />} />
          <Route path="movimientos" element={<MovementsPage />} />
        </Route>
        <Route
          path="*"
          element={
            <div className="state">
              <h1>{es.notFound.title}</h1>
              <p className="muted">{es.notFound.body}</p>
              <Link to="/catalogo" className="button">
                {es.common.back}
              </Link>
            </div>
          }
        />
      </Route>
    </Routes>
  )
}
