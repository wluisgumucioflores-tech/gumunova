import { useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { listMovements, type Movement } from '../../api/inventory'
import { ReceiptIcon } from '../../components/icons'
import { IconLink } from '../../components/IconButton'
import { useRemoteList } from '../../hooks/useRemoteList'
import { es } from '../../i18n/es'
import { formatSignedDecimalEs } from '../../lib/decimal'
import { Pagination } from './Pagination'

const PAGE_SIZE = 25
const t = es.inventory
const m = t.movements

/** "13/11/2026 14:05" in the browser's time zone. */
function formatDateTimeEs(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  const two = (value: number) => String(value).padStart(2, '0')
  return `${two(date.getDate())}/${two(date.getMonth() + 1)}/${date.getFullYear()} ${two(date.getHours())}:${two(date.getMinutes())}`
}

function quantityText(movement: Movement): string {
  const signed = formatSignedDecimalEs(movement.quantity_delta)
  return movement.unit === 'm' ? t.meters(signed) : t.units(signed.replace(/,00$/, ''))
}

export function MovementsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const productId = searchParams.get('producto') ?? ''
  const rollId = searchParams.get('rollo') ?? ''
  const [offset, setOffset] = useState(0)

  const { data, loading, error, reload } = useRemoteList(JSON.stringify([productId, rollId, offset]), (signal) =>
    listMovements({ productId: productId || undefined, rollId: rollId || undefined, limit: PAGE_SIZE, offset }, signal),
  )
  const filtered = productId !== '' || rollId !== ''

  return (
    <>
      {filtered && (
        <p className="notice">
          {rollId ? m.filteredByRoll : m.filteredByProduct}{' '}
          <button
            type="button"
            className="link-button"
            onClick={() => {
              setSearchParams({})
              setOffset(0)
            }}
          >
            {m.clearFilters}
          </button>
        </p>
      )}

      {error ? (
        <div className="state" role="alert">
          <p>
            {t.loadError} {error}
          </p>
          <button type="button" className="button" onClick={reload}>
            {es.common.retry}
          </button>
        </div>
      ) : data === null ? (
        <p className="state" role="status">
          {es.common.loading}
        </p>
      ) : data.items.length === 0 ? (
        <div className="state">
          <h2>{m.emptyTitle}</h2>
          <p className="muted">{m.emptyBody}</p>
          {!filtered && (
            <Link to="/compras/nueva" className="button button-primary">
              {es.purchases.newPurchase}
            </Link>
          )}
        </div>
      ) : (
        <>
          <div className="table-wrap" aria-busy={loading}>
            <table className="table-cards">
              <thead>
                <tr>
                  <th scope="col">{m.columns.date}</th>
                  <th scope="col">{m.columns.type}</th>
                  <th scope="col">{m.columns.product}</th>
                  <th scope="col">{m.columns.roll}</th>
                  <th scope="col" className="numeric">
                    {m.columns.quantity}
                  </th>
                  <th scope="col">{m.columns.reference}</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((movement) => (
                  <tr key={movement.id}>
                    <td data-label={m.columns.date}>{formatDateTimeEs(movement.created_at)}</td>
                    <td data-label={m.columns.type}>{m.types[movement.movement_type] ?? m.unknownType}</td>
                    <td data-label={m.columns.product}>
                      {movement.product.sku} — {movement.product.name}
                    </td>
                    <td data-label={m.columns.roll}>{movement.roll?.code ?? es.common.noValue}</td>
                    <td data-label={m.columns.quantity} className="numeric">{quantityText(movement)}</td>
                    <td data-label={m.columns.reference}>
                      {movement.reference_type === 'purchase' ? (
                        <IconLink to={`/compras/${movement.reference_id}`} label={m.referencePurchase} icon={<ReceiptIcon />} />
                      ) : (
                        es.common.noValue
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination
            summary={m.resultsSummary(data.items.length, data.total)}
            offset={offset}
            pageSize={PAGE_SIZE}
            total={data.total}
            busy={loading}
            onChange={setOffset}
          />
        </>
      )}
    </>
  )
}
