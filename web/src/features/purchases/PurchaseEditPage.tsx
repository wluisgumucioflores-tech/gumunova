import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { isAbortError, messageForApiError } from '../../api/client'
import { listAllActiveProducts, listAllActiveSuppliers } from '../../api/fetchAll'
import { getPurchase, updatePurchase, type Purchase, type PurchaseInput } from '../../api/purchases'
import type { Supplier } from '../../api/suppliers'
import { es } from '../../i18n/es'
import { PurchaseForm, type ProductOption } from './PurchaseForm'
import { purchaseToFormValues } from './formModel'

interface EditData {
  purchase: Purchase
  suppliers: Pick<Supplier, 'id' | 'name'>[]
  pvcProducts: ProductOption[]
  tableclothProducts: ProductOption[]
}

/** Adds what the purchase already uses (maybe deactivated since) so its current choices stay visible. */
function withCurrent<T extends { id: string }>(options: T[], current: T[]): T[] {
  const known = new Set(options.map((item) => item.id))
  return [...options, ...current.filter((item) => !known.has(item.id))]
}

export function PurchaseEditPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [reloadToken, setReloadToken] = useState(0)

  // What the form needs, tagged with the request that produced it, so "loading" is derived.
  const [loaded, setLoaded] = useState<{ key: string; data: EditData | null; error: string | null } | null>(null)
  const requestKey = `${id}#${reloadToken}`
  const current = loaded?.key === requestKey ? loaded : null
  const data = current?.data ?? null
  const loadError = current?.error ?? null

  useEffect(() => {
    const controller = new AbortController()
    Promise.all([
      getPurchase(id, controller.signal),
      listAllActiveSuppliers(controller.signal),
      listAllActiveProducts('pvc', controller.signal),
      listAllActiveProducts('tablecloth', controller.signal),
    ])
      .then(([purchase, suppliers, pvcProducts, tableclothProducts]) => {
        const usedProducts = (type: 'pvc' | 'tablecloth') =>
          purchase.lines.filter((line) => line.product.product_type === type).map((line) => line.product)
        setLoaded({
          key: requestKey,
          error: null,
          data: {
            purchase,
            suppliers: withCurrent(suppliers, purchase.supplier ? [purchase.supplier] : []),
            pvcProducts: withCurrent(pvcProducts, usedProducts('pvc')),
            tableclothProducts: withCurrent(tableclothProducts, usedProducts('tablecloth')),
          },
        })
      })
      .catch((error: unknown) => {
        if (!isAbortError(error)) setLoaded({ key: requestKey, data: null, error: messageForApiError(error) })
      })
    return () => controller.abort()
  }, [id, requestKey])

  async function handleSubmit(input: PurchaseInput) {
    await updatePurchase(id, input)
    navigate('/compras', { state: { text: es.purchases.updatedNotice, tone: 'success' } })
  }

  return (
    <>
      <Link to={`/compras/${id}`} className="back-link">
        {es.purchases.back}
      </Link>
      <header className="page-header">
        <h1>{es.purchases.form.editTitle}</h1>
      </header>

      {loadError ? (
        <div className="state" role="alert">
          <p>
            {es.purchases.detail.loadError} {loadError}
          </p>
          <button type="button" className="button" onClick={() => setReloadToken((token) => token + 1)}>
            {es.common.retry}
          </button>
        </div>
      ) : data === null ? (
        <p className="state" role="status">
          {es.common.loading}
        </p>
      ) : (
        <PurchaseForm
          suppliers={data.suppliers}
          pvcProducts={data.pvcProducts}
          tableclothProducts={data.tableclothProducts}
          initialValues={purchaseToFormValues(data.purchase)}
          itemsLocked={!data.purchase.items_editable}
          onSubmit={handleSubmit}
          onCancel={() => navigate(`/compras/${id}`)}
        />
      )}
    </>
  )
}
