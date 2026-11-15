import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { listSales, type SaleSummary } from '../../api/sales'
import { ReceiptIcon } from '../../components/icons'
import { IconLink } from '../../components/IconButton'
import { Toast } from '../../components/Toast'
import { useNavigationToast } from '../../hooks/useNavigationToast'
import { useRemoteList } from '../../hooks/useRemoteList'
import { es } from '../../i18n/es'
import { formatDateEs, formatDecimalEs } from '../../lib/decimal'
import { Pagination } from '../inventory/Pagination'

const PAGE_SIZE = 25
const SEARCH_DELAY_MS = 300
const t = es.sales

export function SaleListPage() {
  const { toast, dismissToast } = useNavigationToast()

  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
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

  const { data, loading, error, reload } = useRemoteList(JSON.stringify([from, to, query, offset]), (signal) =>
    listSales({ from: from || undefined, to: to || undefined, query, limit: PAGE_SIZE, offset }, signal),
  )
  const hasFilters = from !== '' || to !== '' || query !== ''

  return (
    <>
      {toast && <Toast text={toast.text} tone={toast.tone} onDismiss={dismissToast} />}

      <header className="page-header">
        <div>
          <h1>{t.title}</h1>
          <p className="muted">{t.subtitle}</p>
        </div>
        <Link to="/ventas/nueva" className="button button-primary">
          {t.newSale}
        </Link>
      </header>

      <div className="filters">
        <div className="field">
          <label htmlFor="sale-search">{t.searchLabel}</label>
          <input
            id="sale-search"
            type="search"
            value={searchInput}
            placeholder={t.searchPlaceholder}
            onChange={(event) => setSearchInput(event.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor="sale-from">{t.fromLabel}</label>
          <input
            id="sale-from"
            type="date"
            value={from}
            max={to || undefined}
            onChange={(event) => {
              setFrom(event.target.value)
              setOffset(0)
            }}
          />
        </div>
        <div className="field">
          <label htmlFor="sale-to">{t.toLabel}</label>
          <input
            id="sale-to"
            type="date"
            value={to}
            min={from || undefined}
            onChange={(event) => {
              setTo(event.target.value)
              setOffset(0)
            }}
          />
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
          <h2>{hasFilters ? t.noResultsTitle : t.emptyTitle}</h2>
          <p className="muted">{hasFilters ? t.noResultsBody : t.emptyBody}</p>
          {!hasFilters && (
            <Link to="/ventas/nueva" className="button button-primary">
              {t.newSale}
            </Link>
          )}
        </div>
      ) : (
        <>
          <div className="table-wrap" aria-busy={loading}>
            <table className="table-cards">
              <thead>
                <tr>
                  <th scope="col">{t.columns.date}</th>
                  <th scope="col" className="numeric">
                    {t.columns.lines}
                  </th>
                  <th scope="col" className="numeric">
                    {t.columns.total}
                  </th>
                  <th scope="col" className="numeric">
                    {t.columns.profit}
                  </th>
                  <th scope="col">{t.columns.actions}</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((sale) => (
                  <SaleRow key={sale.id} sale={sale} />
                ))}
              </tbody>
            </table>
          </div>
          <Pagination
            summary={t.resultsSummary(data.items.length, data.total)}
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

function SaleRow({ sale }: { sale: SaleSummary }) {
  const date = formatDateEs(sale.sale_date)
  return (
    <tr>
      <td data-label={t.columns.date}>{date}</td>
      <td data-label={t.columns.lines} className="numeric">{sale.line_count}</td>
      <td data-label={t.columns.total} className="numeric">{t.money(formatDecimalEs(sale.total_amount))}</td>
      <td data-label={t.columns.profit} className="numeric">{t.money(formatDecimalEs(sale.gross_profit))}</td>
      <td data-label={t.columns.actions}>
        <div className="row-actions">
          <IconLink to={`/ventas/${sale.id}`} label={`${t.view} ${date}`} icon={<ReceiptIcon />} />
        </div>
      </td>
    </tr>
  )
}
