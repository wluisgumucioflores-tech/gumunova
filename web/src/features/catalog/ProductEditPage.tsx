import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { ArrowLeftIcon } from '../../components/icons'
import { isAbortError, messageForApiError } from '../../api/client'
import { getProduct, updateProduct, type Product, type ProductInput } from '../../api/products'
import { es } from '../../i18n/es'
import { ImagePanel } from './ImagePanel'
import { ProductForm } from './ProductForm'

export function ProductEditPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()

  const [reloadToken, setReloadToken] = useState(0)

  // The loaded product (or error), tagged with the request that produced it, so "loading"
  // is derived rather than set from inside the effect.
  const [loaded, setLoaded] = useState<{ key: string; product: Product | null; error: string | null } | null>(null)
  const requestKey = `${id}#${reloadToken}`
  const current = loaded?.key === requestKey ? loaded : null
  const product = current?.product ?? null
  const loadError = current?.error ?? null

  useEffect(() => {
    const controller = new AbortController()
    getProduct(id, controller.signal)
      .then((result) => setLoaded({ key: requestKey, product: result, error: null }))
      .catch((error: unknown) => {
        if (!isAbortError(error)) setLoaded({ key: requestKey, product: null, error: messageForApiError(error) })
      })
    return () => controller.abort()
  }, [id, requestKey])

  async function handleSubmit(input: ProductInput) {
    await updateProduct(id, input)
    navigate('/catalogo', { state: { text: es.form.savedNotice, tone: 'success' } })
  }

  return (
    <>
      <Link to="/catalogo" className="back-link">
        <ArrowLeftIcon />
        {es.common.back}
      </Link>
      <header className="page-header">
        <div>
          <div className="page-title">
            <h1>{es.form.editTitle}</h1>
            {product && <span className="badge-type">{es.catalog.types[product.product_type]}</span>}
          </div>
          <p className="page-subtitle">{es.form.editSubtitle}</p>
        </div>
      </header>

      {loadError ? (
        <div className="state" role="alert">
          <p>
            {es.form.loadError} {loadError}
          </p>
          <button type="button" className="button" onClick={() => setReloadToken((token) => token + 1)}>
            {es.common.retry}
          </button>
        </div>
      ) : product === null ? (
        <p className="state" role="status">
          {es.common.loading}
        </p>
      ) : (
        <div className="edit-layout">
          <ProductForm product={product} onSubmit={handleSubmit} onCancel={() => navigate('/catalogo')} />
          <ImagePanel
            product={product}
            onChange={(updated) => setLoaded({ key: requestKey, product: updated, error: null })}
          />
        </div>
      )}
    </>
  )
}
