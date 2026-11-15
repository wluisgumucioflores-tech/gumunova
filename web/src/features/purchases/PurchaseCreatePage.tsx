import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { isAbortError, messageForApiError } from '../../api/client'
import { listAllActiveProducts, listAllActiveSuppliers } from '../../api/fetchAll'
import type { Product } from '../../api/products'
import { registerPurchase, type PurchaseInput } from '../../api/purchases'
import type { Supplier } from '../../api/suppliers'
import { es } from '../../i18n/es'
import { PurchaseForm } from './PurchaseForm'

interface FormData {
  suppliers: Supplier[]
  pvcProducts: Product[]
  tableclothProducts: Product[]
}

export function PurchaseCreatePage() {
  const navigate = useNavigate()
  const [reloadToken, setReloadToken] = useState(0)

  // What the form needs, tagged with the request that produced it, so "loading" is derived
  // rather than set from inside the effect.
  const [loaded, setLoaded] = useState<{ key: number; data: FormData | null; error: string | null } | null>(null)
  const current = loaded?.key === reloadToken ? loaded : null
  const data = current?.data ?? null
  const loadError = current?.error ?? null

  useEffect(() => {
    const controller = new AbortController()
    Promise.all([
      listAllActiveSuppliers(controller.signal),
      listAllActiveProducts('pvc', controller.signal),
      listAllActiveProducts('tablecloth', controller.signal),
    ])
      .then(([suppliers, pvcProducts, tableclothProducts]) =>
        setLoaded({ key: reloadToken, data: { suppliers, pvcProducts, tableclothProducts }, error: null }),
      )
      .catch((error: unknown) => {
        if (!isAbortError(error)) setLoaded({ key: reloadToken, data: null, error: messageForApiError(error) })
      })
    return () => controller.abort()
  }, [reloadToken])

  async function handleSubmit(input: PurchaseInput, idempotencyKey: string) {
    await registerPurchase(input, idempotencyKey)
    navigate('/compras', { state: { text: es.purchases.createdNotice, tone: 'success' } })
  }

  return (
    <>
      <Link to="/compras" className="back-link">
        {es.purchases.back}
      </Link>
      <header className="page-header">
        <h1>{es.purchases.form.title}</h1>
      </header>

      {loadError ? (
        <div className="state" role="alert">
          <p>
            {es.purchases.form.loadError} {loadError}
          </p>
          <button type="button" className="button" onClick={() => setReloadToken((token) => token + 1)}>
            {es.common.retry}
          </button>
        </div>
      ) : data === null ? (
        <p className="state" role="status">
          {es.common.loading}
        </p>
      ) : data.pvcProducts.length === 0 && data.tableclothProducts.length === 0 ? (
        <div className="state">
          <p>{es.purchases.form.noProducts}</p>
          <Link to="/catalogo/nuevo" className="button button-primary">
            {es.catalog.newProduct}
          </Link>
        </div>
      ) : (
        <PurchaseForm
          suppliers={data.suppliers}
          pvcProducts={data.pvcProducts}
          tableclothProducts={data.tableclothProducts}
          onSubmit={handleSubmit}
          onCancel={() => navigate('/compras')}
        />
      )}
    </>
  )
}
