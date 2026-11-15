import { Link, useParams } from 'react-router'
import { getSale, type Sale, type SaleLine } from '../../api/sales'
import { useRemoteList } from '../../hooks/useRemoteList'
import { es } from '../../i18n/es'
import { formatDateEs, formatDecimalEs, formatUnitsEs } from '../../lib/decimal'

const t = es.sales.detail
const money = (amount: string) => es.sales.money(formatDecimalEs(amount))

export function SaleDetailPage() {
  const { id = '' } = useParams()
  const { data: sale, error, reload } = useRemoteList(id, (signal) => getSale(id, signal))

  return (
    <>
      <Link to="/ventas" className="back-link">
        {es.sales.back}
      </Link>
      <header className="page-header">
        <h1>{t.title}</h1>
      </header>

      {error ? (
        <div className="state" role="alert">
          <p>
            {t.loadError} {error}
          </p>
          <button type="button" className="button" onClick={reload}>
            {es.common.retry}
          </button>
        </div>
      ) : sale === null ? (
        <p className="state" role="status">
          {es.common.loading}
        </p>
      ) : (
        <SaleView sale={sale} />
      )}
    </>
  )
}

function quantityText(line: SaleLine): string {
  if (line.unit === 'm') return `${formatDecimalEs(line.quantity)} m`
  if (line.unit === 'roll') return `1 ${t.units.roll}`
  return `${formatUnitsEs(line.quantity)} ${t.units.unit}`
}

function SaleView({ sale }: { sale: Sale }) {
  return (
    <div className="detail">
      <section className="panel" aria-label={t.title}>
        <dl className="facts">
          <div>
            <dt>{t.date}</dt>
            <dd>{formatDateEs(sale.sale_date)}</dd>
          </div>
          <div>
            <dt>{t.statusLabel}</dt>
            <dd>
              <span className="badge badge-active">{t.status[sale.status]}</span>
            </dd>
          </div>
          <div>
            <dt>{t.cost}</dt>
            <dd>{money(sale.total_cost)}</dd>
          </div>
          <div>
            <dt>{t.profit}</dt>
            <dd>{money(sale.gross_profit)}</dd>
          </div>
          <div className="summary-total">
            <dt>{t.total}</dt>
            <dd>{money(sale.total_amount)}</dd>
          </div>
        </dl>
        {sale.notes && (
          <p>
            <strong>{t.notes}:</strong> {sale.notes}
          </p>
        )}
        <p className="field-hint">{t.immutableNote}</p>
      </section>

      {sale.lines.map((line) => (
        <section key={line.id} className="panel" aria-label={t.lineHeading(line.number, line.product.name)}>
          <h2>{t.lineHeading(line.number, line.product.name)}</h2>
          <p className="muted">
            {t.sku}: <span className="mono">{line.product.sku}</span> · {es.sales.kinds[line.kind]}
            {line.roll && (
              <>
                {' '}
                · {t.roll}: <span className="mono">{line.roll.code}</span>
              </>
            )}
          </p>
          <dl className="facts">
            <div>
              <dt>{t.quantity}</dt>
              <dd>{quantityText(line)}</dd>
            </div>
            <div>
              <dt>{t.unitPrice}</dt>
              <dd>{money(line.unit_price)}</dd>
            </div>
            <div>
              <dt>{t.referencePrice}</dt>
              <dd>{money(line.reference_price)}</dd>
            </div>
            <div>
              <dt>{t.lineTotal}</dt>
              <dd>{money(line.line_total)}</dd>
            </div>
            <div>
              <dt>{t.lineCost}</dt>
              <dd>{money(line.cost_of_sale)}</dd>
            </div>
            <div>
              <dt>{t.lineProfit}</dt>
              <dd>{money(line.gross_profit)}</dd>
            </div>
          </dl>
        </section>
      ))}
    </div>
  )
}
