import { Link, useNavigate } from 'react-router'
import { createSupplier, type SupplierInput } from '../../api/suppliers'
import { es } from '../../i18n/es'
import { SupplierForm } from './SupplierForm'

export function SupplierCreatePage() {
  const navigate = useNavigate()

  async function handleSubmit(input: SupplierInput) {
    await createSupplier(input)
    navigate('/proveedores', { state: { text: es.suppliers.createdNotice, tone: 'success' } })
  }

  return (
    <>
      <Link to="/proveedores" className="back-link">
        {es.suppliers.back}
      </Link>
      <header className="page-header">
        <h1>{es.suppliers.createTitle}</h1>
      </header>
      <SupplierForm onSubmit={handleSubmit} onCancel={() => navigate('/proveedores')} />
    </>
  )
}
