import { Link, useNavigate } from 'react-router'
import { listAllActiveProducts, listAllAvailableRolls, listAllTableclothStock } from '../../api/fetchAll'
import { registerSale, type SaleInput } from '../../api/sales'
import { useRemoteList } from '../../hooks/useRemoteList'
import { es } from '../../i18n/es'
import { SaleForm } from './SaleForm'

export function SaleCreatePage() {
  const navigate = useNavigate()

  const { data, loading, error, reload } = useRemoteList('sale-form-data', async (signal) => {
    const [rolls, tablecloths, pvcProducts, tableclothProducts] = await Promise.all([
      listAllAvailableRolls(signal),
      listAllTableclothStock(signal),
      listAllActiveProducts('pvc', signal),
      listAllActiveProducts('tablecloth', signal),
    ])
    return { rolls, tablecloths, products: [...pvcProducts, ...tableclothProducts] }
  })

  async function handleSubmit(input: SaleInput, idempotencyKey: string) {
    await registerSale(input, idempotencyKey)
    navigate('/ventas', { state: { text: es.sales.createdNotice, tone: 'success' } })
  }

  const nothingToSell = data !== null && data.rolls.length === 0 && data.tablecloths.every((item) => Number(item.quantity.split('.')[0]) === 0)

  return (
    <>
      <Link to="/ventas" className="back-link">
        {es.sales.back}
      </Link>
      <header className="page-header">
        <h1>{es.sales.form.title}</h1>
      </header>

      {error ? (
        <div className="state" role="alert">
          <p>
            {es.sales.form.loadError} {error}
          </p>
          <button type="button" className="button" onClick={reload}>
            {es.common.retry}
          </button>
        </div>
      ) : data === null || loading ? (
        <p className="state" role="status">
          {es.common.loading}
        </p>
      ) : nothingToSell ? (
        <div className="state">
          <p>{es.sales.form.nothingToSell}</p>
          <Link to="/compras/nueva" className="button button-primary">
            {es.purchases.newPurchase}
          </Link>
        </div>
      ) : (
        <SaleForm
          rolls={data.rolls}
          tablecloths={data.tablecloths}
          products={data.products}
          onSubmit={handleSubmit}
          onCancel={() => navigate('/ventas')}
        />
      )}
    </>
  )
}
