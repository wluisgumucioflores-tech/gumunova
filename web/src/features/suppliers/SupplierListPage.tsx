import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { isAbortError, messageForApiError } from '../../api/client'
import {
  activateSupplier,
  deactivateSupplier,
  listSuppliers,
  type Supplier,
  type SupplierList,
} from '../../api/suppliers'
import { EyeIcon, EyeOffIcon, PencilIcon } from '../../components/icons'
import { IconButton, IconLink } from '../../components/IconButton'
import { Toast } from '../../components/Toast'
import { useNavigationToast } from '../../hooks/useNavigationToast'
import { es } from '../../i18n/es'

const PAGE_SIZE = 25
const SEARCH_DELAY_MS = 300

type StatusFilter = 'all' | 'active' | 'inactive'

const t = es.suppliers

export function SupplierListPage() {
  const { toast, dismissToast } = useNavigationToast()

  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [searchInput, setSearchInput] = useState('')
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)
  const [reloadToken, setReloadToken] = useState(0)

  // The last answer, tagged with the request that produced it. "Loading" is derived from
  // whether that answer belongs to the current filters.
  const [response, setResponse] = useState<{ key: string; data: SupplierList | null; error: string | null } | null>(null)
  const requestKey = JSON.stringify([statusFilter, query, offset, reloadToken])
  const loading = response?.key !== requestKey
  const data = response?.data ?? null
  const loadError = response?.key === requestKey ? response.error : null

  const [confirmingId, setConfirmingId] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(searchInput.trim())
      setOffset(0)
    }, SEARCH_DELAY_MS)
    return () => clearTimeout(timer)
  }, [searchInput])

  useEffect(() => {
    const controller = new AbortController()
    listSuppliers(
      {
        isActive: statusFilter === 'all' ? undefined : statusFilter === 'active',
        query,
        limit: PAGE_SIZE,
        offset,
      },
      controller.signal,
    )
      .then((result) => setResponse({ key: requestKey, data: result, error: null }))
      .catch((error: unknown) => {
        if (isAbortError(error)) return
        setResponse({ key: requestKey, data: null, error: messageForApiError(error) })
      })
    return () => controller.abort()
  }, [statusFilter, query, offset, reloadToken, requestKey])

  const hasFilters = statusFilter !== 'all' || query !== ''

  async function changeActive(supplier: Supplier, activate: boolean) {
    setBusyId(supplier.id)
    setActionError(null)
    try {
      await (activate ? activateSupplier(supplier.id) : deactivateSupplier(supplier.id))
      setConfirmingId(null)
      setReloadToken((token) => token + 1)
    } catch (error) {
      setActionError(`${t.actionError} ${messageForApiError(error)}`)
    } finally {
      setBusyId(null)
    }
  }

  return (
    <>
      {toast && <Toast text={toast.text} tone={toast.tone} onDismiss={dismissToast} />}

      <header className="page-header">
        <div>
          <h1>{t.title}</h1>
          <p className="muted">{t.subtitle}</p>
        </div>
        <Link to="/proveedores/nuevo" className="button button-primary">
          {t.newSupplier}
        </Link>
      </header>

      <div className="filters filters-two">
        <div className="field">
          <label htmlFor="supplier-search">{t.searchLabel}</label>
          <input
            id="supplier-search"
            type="search"
            value={searchInput}
            placeholder={t.searchPlaceholder}
            onChange={(event) => setSearchInput(event.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor="supplier-status">{es.common.statusFilterLabel}</label>
          <select
            id="supplier-status"
            value={statusFilter}
            onChange={(event) => {
              setStatusFilter(event.target.value as StatusFilter)
              setOffset(0)
            }}
          >
            <option value="all">{es.common.allStatuses}</option>
            <option value="active">{es.common.activeOnly}</option>
            <option value="inactive">{es.common.inactiveOnly}</option>
          </select>
        </div>
      </div>

      {actionError && (
        <p className="form-error" role="alert">
          {actionError}
        </p>
      )}

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
            <Link to="/proveedores/nuevo" className="button button-primary">
              {t.newSupplier}
            </Link>
          )}
        </div>
      ) : (
        <>
          <div className="table-wrap" aria-busy={loading}>
            <table className="table-cards">
              <thead>
                <tr>
                  <th scope="col">{t.columns.name}</th>
                  <th scope="col">{t.columns.phone}</th>
                  <th scope="col">{t.columns.email}</th>
                  <th scope="col">{t.columns.status}</th>
                  <th scope="col">{t.columns.actions}</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((supplier) => (
                  <SupplierRow
                    key={supplier.id}
                    supplier={supplier}
                    confirming={confirmingId === supplier.id}
                    busy={busyId === supplier.id}
                    onAskDeactivate={() => setConfirmingId(supplier.id)}
                    onCancelDeactivate={() => setConfirmingId(null)}
                    onDeactivate={() => changeActive(supplier, false)}
                    onActivate={() => changeActive(supplier, true)}
                  />
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

interface SupplierRowProps {
  supplier: Supplier
  confirming: boolean
  busy: boolean
  onAskDeactivate: () => void
  onCancelDeactivate: () => void
  onDeactivate: () => void
  onActivate: () => void
}

function SupplierRow({
  supplier,
  confirming,
  busy,
  onAskDeactivate,
  onCancelDeactivate,
  onDeactivate,
  onActivate,
}: SupplierRowProps) {
  return (
    <tr className={supplier.is_active ? undefined : 'is-inactive'}>
      <td data-label={t.columns.name}>{supplier.name}</td>
      <td data-label={t.columns.phone}>{supplier.phone || es.common.noValue}</td>
      <td data-label={t.columns.email}>{supplier.email || es.common.noValue}</td>
      <td data-label={t.columns.status}>
        <span className={supplier.is_active ? 'badge badge-active' : 'badge badge-inactive'}>
          {supplier.is_active ? es.common.statusActive : es.common.statusInactive}
        </span>
      </td>
      <td data-label={t.columns.actions}>
        {confirming ? (
          <div className="confirm" role="group" aria-label={t.confirmDeactivate(supplier.name)}>
            <p>{t.confirmDeactivate(supplier.name)}</p>
            <button type="button" className="button button-danger" disabled={busy} onClick={onDeactivate}>
              {t.confirmDeactivateYes}
            </button>
            <button type="button" className="button" disabled={busy} onClick={onCancelDeactivate}>
              {es.common.cancel}
            </button>
          </div>
        ) : (
          <div className="row-actions">
            <IconLink
              to={`/proveedores/${supplier.id}/editar`}
              label={`${es.common.edit} ${supplier.name}`}
              icon={<PencilIcon />}
            />
            {supplier.is_active ? (
              <IconButton
                label={`${t.deactivate} ${supplier.name}`}
                icon={<EyeOffIcon />}
                disabled={busy}
                onClick={onAskDeactivate}
              />
            ) : (
              <IconButton
                label={`${t.activate} ${supplier.name}`}
                icon={<EyeIcon />}
                disabled={busy}
                onClick={onActivate}
              />
            )}
          </div>
        )}
      </td>
    </tr>
  )
}
