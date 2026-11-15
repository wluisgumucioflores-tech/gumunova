import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { listStock, type StockItem } from '../../api/inventory'
import type { ProductType } from '../../api/products'
import { ListIcon, RollIcon } from '../../components/icons'
import { IconLink } from '../../components/IconButton'
import { useRemoteList } from '../../hooks/useRemoteList'
import { es } from '../../i18n/es'
import { formatDecimalEs, formatUnitsEs } from '../../lib/decimal'
import { Pagination } from './Pagination'

const PAGE_SIZE = 25
const SEARCH_DELAY_MS = 300
const t = es.inventory
const s = t.stock

function quantityText(item: StockItem, amount: string): string {
  return item.unit === 'm' ? t.meters(formatDecimalEs(amount)) : t.units(formatUnitsEs(amount))
}

export function StockPage() {
  const [type, setType] = useState<ProductType | ''>('')
  const [onlyLow, setOnlyLow] = useState(false)
  const [searchInput, setSearchInput] = useState('')
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)

  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(searchInput.trim())
      setOffset(0)
    }, SEARCH_DELAY_MS)
    return () => clearTimeout(timer)
  }, [searchInput])

  const { data, loading, error, reload } = useRemoteList(JSON.stringify([type, onlyLow, query, offset]), (signal) =>
    listStock({ type: type || undefined, onlyLow, query, limit: PAGE_SIZE, offset }, signal),
  )
  const hasFilters = type !== '' || onlyLow || query !== ''

  return (
    <>
      <div className="filters filters-four">
        <div className="field">
          <label htmlFor="stock-search">{s.searchLabel}</label>
          <input id="stock-search" type="search" value={searchInput} onChange={(event) => setSearchInput(event.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="stock-type">{s.typeLabel}</label>
          <select
            id="stock-type"
            value={type}
            onChange={(event) => {
              setType(event.target.value as ProductType | '')
              setOffset(0)
            }}
          >
            <option value="">{s.allTypes}</option>
            <option value="pvc">{es.catalog.types.pvc}</option>
            <option value="tablecloth">{es.catalog.types.tablecloth}</option>
          </select>
        </div>
        <div className="field field-check">
          <label htmlFor="stock-low">
            <input
              id="stock-low"
              type="checkbox"
              checked={onlyLow}
              onChange={(event) => {
                setOnlyLow(event.target.checked)
                setOffset(0)
              }}
            />
            {s.onlyLowLabel}
          </label>
        </div>
      </div>

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
          <h2>{hasFilters ? s.noResultsTitle : s.emptyTitle}</h2>
          <p className="muted">{hasFilters ? s.noResultsBody : s.emptyBody}</p>
          {!hasFilters && (
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
                  <th scope="col">{s.columns.sku}</th>
                  <th scope="col">{s.columns.name}</th>
                  <th scope="col">{s.columns.type}</th>
                  <th scope="col" className="numeric">
                    {s.columns.quantity}
                  </th>
                  <th scope="col" className="numeric">
                    {s.columns.minimum}
                  </th>
                  <th scope="col">{s.columns.status}</th>
                  <th scope="col" className="numeric">
                    {s.columns.rolls}
                  </th>
                  <th scope="col" className="numeric">
                    {s.columns.value}
                  </th>
                  <th scope="col" className="numeric">
                    {s.columns.averageCost}
                  </th>
                  <th scope="col">{s.columns.actions}</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((item) => (
                  <StockRow key={item.product_id} item={item} />
                ))}
              </tbody>
            </table>
          </div>
          <p className="field-hint">{s.valueHint}</p>
          <Pagination
            summary={s.resultsSummary(data.items.length, data.total)}
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

function StockRow({ item }: { item: StockItem }) {
  const hasAlert = Number(item.minimum.replace('.', '')) > 0
  return (
    <tr>
      <td data-label={s.columns.sku}>{item.sku}</td>
      <td data-label={s.columns.name}>{item.name}</td>
      <td data-label={s.columns.type}>{es.catalog.types[item.product_type]}</td>
      <td data-label={s.columns.quantity} className="numeric">{quantityText(item, item.quantity)}</td>
      <td data-label={s.columns.minimum} className="numeric">{hasAlert ? quantityText(item, item.minimum) : es.common.noValue}</td>
      <td data-label={s.columns.status}>
        {item.is_low ? (
          <span className="badge badge-warning">{s.lowBadge}</span>
        ) : hasAlert ? (
          <span className="badge badge-active">{s.okBadge}</span>
        ) : (
          <span className="muted">{s.noAlert}</span>
        )}
      </td>
      <td data-label={s.columns.rolls} className="numeric">{item.roll_count ?? es.common.noValue}</td>
      <td data-label={s.columns.value} className="numeric">{t.money(formatDecimalEs(item.stock_value))}</td>
      <td data-label={s.columns.averageCost} className="numeric">{item.average_cost ? t.money(formatDecimalEs(item.average_cost)) : es.common.noValue}</td>
      <td data-label={s.columns.actions}>
        <div className="row-actions">
          {item.product_type === 'pvc' && (
            <IconLink to={`/inventario/rollos?producto=${item.product_id}`} label={`${s.viewRolls} ${item.sku}`} icon={<RollIcon />} />
          )}
          <IconLink to={`/inventario/movimientos?producto=${item.product_id}`} label={`${s.viewMovements} ${item.sku}`} icon={<ListIcon />} />
        </div>
      </td>
    </tr>
  )
}
