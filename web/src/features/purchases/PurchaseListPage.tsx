import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { isAbortError, messageForApiError } from '../../api/client'
import { listAllSuppliers } from '../../api/fetchAll'
import { listPurchases, type PurchaseList, type PurchaseSummary } from '../../api/purchases'
import type { Supplier } from '../../api/suppliers'
import { PencilIcon, ReceiptIcon } from '../../components/icons'
import { IconLink } from '../../components/IconButton'
import { Toast } from '../../components/Toast'
import { useNavigationToast } from '../../hooks/useNavigationToast'
import { es } from '../../i18n/es'
import { formatDateEs, formatDecimalEs } from '../../lib/decimal'

const PAGE_SIZE = 25
const SEARCH_DELAY_MS = 300

const t = es.purchases

export function PurchaseListPage() {
  const { toast, dismissToast } = useNavigationToast()

  const [supplierFilter, setSupplierFilter] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)
  const [reloadToken, setReloadToken] = useState(0)
  const [suppliers, setSuppliers] = useState<Supplier[]>([])

  // The last answer, tagged with the request that produced it. "Loading" is derived from
  // whether that answer belongs to the current filters.
  const [response, setResponse] = useState<{ key: string; data: PurchaseList | null; error: string | null } | null>(null)
  const requestKey = JSON.stringify([supplierFilter, from, to, query, offset, reloadToken])
  const loading = response?.key !== requestKey
  const data = response?.data ?? null
  const loadError = response?.key === requestKey ? response.error : null

  useEffect(() => {
    const controller = new AbortController()
    listAllSuppliers(controller.signal)
      .then(setSuppliers)
      .catch(() => {
        // The filter is a convenience; the list itself still works without its options.
      })
    return () => controller.abort()
  }, [])

  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(searchInput.trim())
      setOffset(0)
    }, SEARCH_DELAY_MS)
    return () => clearTimeout(timer)
  }, [searchInput])

  useEffect(() => {
    const controller = new AbortController()
    listPurchases(
      { supplierId: supplierFilter || undefined, from: from || undefined, to: to || undefined, query, limit: PAGE_SIZE, offset },
      controller.signal,
    )
      .then((result) => setResponse({ key: requestKey, data: result, error: null }))
      .catch((error: unknown) => {
        if (isAbortError(error)) return
        setResponse({ key: requestKey, data: null, error: messageForApiError(error) })
      })
    return () => controller.abort()
  }, [supplierFilter, from, to, query, offset, reloadToken, requestKey])

  const hasFilters = supplierFilter !== '' || from !== '' || to !== '' || query !== ''

  return (
    <>
      {toast && <Toast text={toast.text} tone={toast.tone} onDismiss={dismissToast} />}

      <header className="page-header">
        <div>
          <h1>{t.title}</h1>
          <p className="muted">{t.subtitle}</p>
        </div>
        <Link to="/compras/nueva" className="button button-primary">
          {t.newPurchase}
        </Link>
      </header>

      <div className="filters filters-four">
        <div className="field">
          <label htmlFor="purchase-search">{t.searchLabel}</label>
          <input
            id="purchase-search"
            type="search"
            value={searchInput}
            placeholder={t.searchPlaceholder}
            onChange={(event) => setSearchInput(event.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor="purchase-supplier-filter">{t.supplierFilterLabel}</label>
          <select
            id="purchase-supplier-filter"
            value={supplierFilter}
            onChange={(event) => {
              setSupplierFilter(event.target.value)
              setOffset(0)
            }}
          >
            <option value="">{t.allSuppliers}</option>
            {suppliers.map((supplier) => (
              <option key={supplier.id} value={supplier.id}>
                {supplier.name}
              </option>
            ))}
          </select>
        </div>
        <div className="field">
          <label htmlFor="purchase-from">{t.fromLabel}</label>
          <input
            id="purchase-from"
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
          <label htmlFor="purchase-to">{t.toLabel}</label>
          <input
            id="purchase-to"
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

      {loadError ? (
        <div className="state" role="alert">
          <p>
            {t.loadError} {loadError}
          </p>
          <button type="button" className="button" onClick={() => setReloadToken((token) => token + 1)}>
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
            <Link to="/compras/nueva" className="button button-primary">
              {t.newPurchase}
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
                  <th scope="col">{t.columns.supplier}</th>
                  <th scope="col">{t.columns.reference}</th>
                  <th scope="col" className="numeric">
                    {t.columns.lines}
                  </th>
                  <th scope="col" className="numeric">
                    {t.columns.total}
                  </th>
                  <th scope="col">{t.columns.actions}</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((purchase) => (
                  <PurchaseRow key={purchase.id} purchase={purchase} />
                ))}
              </tbody>
            </table>
          </div>

          <footer className="pagination">
            <p className="muted" aria-live="polite">
              {t.resultsSummary(data.items.length, data.total)}
            </p>
            <div className="pagination-buttons">
              <button
                type="button"
                className="button"
                disabled={offset === 0 || loading}
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              >
                {es.common.previous}
              </button>
              <button
                type="button"
                className="button"
                disabled={offset + PAGE_SIZE >= data.total || loading}
                onClick={() => setOffset(offset + PAGE_SIZE)}
              >
                {es.common.next}
              </button>
            </div>
          </footer>
        </>
      )}
    </>
  )
}

function PurchaseRow({ purchase }: { purchase: PurchaseSummary }) {
  const date = formatDateEs(purchase.purchase_date)
  const supplierName = purchase.supplier?.name ?? t.noSupplier
  return (
    <tr>
      <td data-label={t.columns.date}>{date}</td>
      <td data-label={t.columns.supplier}>{supplierName}</td>
      <td data-label={t.columns.reference}>{purchase.document_reference || es.common.noValue}</td>
      <td data-label={t.columns.lines} className="numeric">{purchase.line_count}</td>
      <td data-label={t.columns.total} className="numeric">{t.money(formatDecimalEs(purchase.total_amount))}</td>
      <td data-label={t.columns.actions}>
        <div className="row-actions">
          <IconLink
            to={`/compras/${purchase.id}`}
            label={`${t.view} ${purchase.document_reference || `${supplierName} ${date}`}`}
            icon={<ReceiptIcon />}
          />
          <IconLink
            to={`/compras/${purchase.id}/editar`}
            label={`${t.edit} ${purchase.document_reference || `${supplierName} ${date}`}`}
            icon={<PencilIcon />}
          />
        </div>
      </td>
    </tr>
  )
}
