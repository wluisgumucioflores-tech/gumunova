import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { isAbortError, messageForApiError } from '../../api/client'
import { getSupplier, updateSupplier, type Supplier, type SupplierInput } from '../../api/suppliers'
import { es } from '../../i18n/es'
import { SupplierForm } from './SupplierForm'

export function SupplierEditPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [reloadToken, setReloadToken] = useState(0)

  // The loaded supplier (or error), tagged with the request that produced it, so "loading"
  // is derived rather than set from inside the effect.
  const [loaded, setLoaded] = useState<{ key: string; supplier: Supplier | null; error: string | null } | null>(null)
  const requestKey = `${id}#${reloadToken}`
  const current = loaded?.key === requestKey ? loaded : null
  const supplier = current?.supplier ?? null
  const loadError = current?.error ?? null

  useEffect(() => {
    const controller = new AbortController()
    getSupplier(id, controller.signal)
      .then((result) => setLoaded({ key: requestKey, supplier: result, error: null }))
      .catch((error: unknown) => {
        if (!isAbortError(error)) setLoaded({ key: requestKey, supplier: null, error: messageForApiError(error) })
      })
    return () => controller.abort()
  }, [id, requestKey])

  async function handleSubmit(input: SupplierInput) {
    await updateSupplier(id, input)
    navigate('/proveedores', { state: { text: es.suppliers.savedNotice, tone: 'success' } })
  }

  return (
    <>
      <Link to="/proveedores" className="back-link">
        {es.suppliers.back}
      </Link>
      <header className="page-header">
        <h1>{es.suppliers.editTitle}</h1>
      </header>

      {loadError ? (
        <div className="state" role="alert">
          <p>
            {es.suppliers.formLoadError} {loadError}
          </p>
          <button type="button" className="button" onClick={() => setReloadToken((token) => token + 1)}>
            {es.common.retry}
          </button>
        </div>
      ) : supplier === null ? (
        <p className="state" role="status">
          {es.common.loading}
        </p>
      ) : (
        <SupplierForm supplier={supplier} onSubmit={handleSubmit} onCancel={() => navigate('/proveedores')} />
      )}
    </>
  )
}
