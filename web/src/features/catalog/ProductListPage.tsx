import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { isAbortError, messageForApiError } from '../../api/client'
import {
  activateProduct,
  deactivateProduct,
  listProducts,
  resetProductLabelCode,
  type Product,
  type ProductList,
  type ProductType,
} from '../../api/products'
import { EyeIcon, EyeOffIcon, PencilIcon, RefreshIcon } from '../../components/icons'
import { IconButton, IconLink } from '../../components/IconButton'
import { Toast } from '../../components/Toast'
import { useNavigationToast } from '../../hooks/useNavigationToast'
import { formatDecimalEs } from '../../lib/decimal'
import { es } from '../../i18n/es'

const PAGE_SIZE = 25
const SEARCH_DELAY_MS = 300

type TypeFilter = ProductType | 'all'
type StatusFilter = 'all' | 'active' | 'inactive'

export function ProductListPage() {
  const { toast, dismissToast, showToast } = useNavigationToast()

  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [searchInput, setSearchInput] = useState('')
  const [query, setQuery] = useState('')
  const [offset, setOffset] = useState(0)
  const [reloadToken, setReloadToken] = useState(0)

  // The last answer, tagged with the request that produced it. "Loading" is derived from
  // whether that answer belongs to the current filters, so no state is set inside the effect itself.
  const [response, setResponse] = useState<{ key: string; data: ProductList | null; error: string | null } | null>(null)
  const requestKey = JSON.stringify([typeFilter, statusFilter, query, offset, reloadToken])
  const loading = response?.key !== requestKey
  const data = response?.data ?? null
  const loadError = response?.key === requestKey ? response.error : null

  // Products ticked for printing labels; the selection survives changing page and filters.
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set())
  const [confirmingId, setConfirmingId] = useState<string | null>(null)
  const [resettingId, setResettingId] = useState<string | null>(null)
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
    listProducts(
      {
        type: typeFilter === 'all' ? undefined : typeFilter,
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
  }, [typeFilter, statusFilter, query, offset, reloadToken, requestKey])

  const hasFilters = typeFilter !== 'all' || statusFilter !== 'all' || query !== ''

  function toggleSelected(id: string, on: boolean) {
    setSelected((current) => {
      const next = new Set(current)
      if (on) next.add(id)
      else next.delete(id)
      return next
    })
  }

  function togglePage(items: Product[], on: boolean) {
    setSelected((current) => {
      const next = new Set(current)
      for (const item of items) {
        if (on) next.add(item.id)
        else next.delete(item.id)
      }
      return next
    })
  }

  async function resetCode(product: Product) {
    setBusyId(product.id)
    setActionError(null)
    try {
      await resetProductLabelCode(product.id)
      setResettingId(null)
      showToast(es.catalog.labels.resetDone(product.name))
    } catch (error) {
      setActionError(`${es.catalog.labels.resetError} ${messageForApiError(error)}`)
    } finally {
      setBusyId(null)
    }
  }

  async function changeActive(product: Product, activate: boolean) {
    setBusyId(product.id)
    setActionError(null)
    try {
      await (activate ? activateProduct(product.id) : deactivateProduct(product.id))
      setConfirmingId(null)
      setReloadToken((token) => token + 1)
    } catch (error) {
      setActionError(`${es.catalog.actionError} ${messageForApiError(error)}`)
    } finally {
      setBusyId(null)
    }
  }

  return (
    <>
      <header className="page-header">
        <div>
          <h1>{es.catalog.title}</h1>
          <p className="muted">{es.catalog.subtitle}</p>
        </div>
        <div className="header-actions">
          <Link to="/catalogo/etiquetas?todos=1" className="button">
            {es.catalog.labels.openAll}
          </Link>
          <Link to="/catalogo/nuevo" className="button button-primary">
            {es.catalog.newProduct}
          </Link>
        </div>
      </header>

      {selected.size > 0 && (
        <div className="selection-bar" role="group" aria-label={es.catalog.labels.selectedCount(selected.size)}>
          <span>{es.catalog.labels.selectedCount(selected.size)}</span>
          <Link to={`/catalogo/etiquetas?ids=${[...selected].join(',')}`} className="button button-primary">
            {es.catalog.labels.open}
          </Link>
          <button type="button" className="button" onClick={() => setSelected(new Set())}>
            {es.catalog.labels.clearSelection}
          </button>
        </div>
      )}

      <div className="filters">
        <div className="field">
          <label htmlFor="catalog-search">{es.catalog.searchLabel}</label>
          <input
            id="catalog-search"
            type="search"
            value={searchInput}
            placeholder={es.catalog.searchPlaceholder}
            onChange={(event) => setSearchInput(event.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor="catalog-type">{es.catalog.typeFilterLabel}</label>
          <select
            id="catalog-type"
            value={typeFilter}
            onChange={(event) => {
              setTypeFilter(event.target.value as TypeFilter)
              setOffset(0)
            }}
          >
            <option value="all">{es.catalog.allTypes}</option>
            <option value="pvc">{es.catalog.types.pvc}</option>
            <option value="tablecloth">{es.catalog.types.tablecloth}</option>
          </select>
        </div>
        <div className="field">
          <label htmlFor="catalog-status">{es.catalog.statusFilterLabel}</label>
          <select
            id="catalog-status"
            value={statusFilter}
            onChange={(event) => {
              setStatusFilter(event.target.value as StatusFilter)
              setOffset(0)
            }}
          >
            <option value="all">{es.catalog.allStatuses}</option>
            <option value="active">{es.catalog.activeOnly}</option>
            <option value="inactive">{es.catalog.inactiveOnly}</option>
          </select>
        </div>
      </div>

      {toast && <Toast text={toast.text} tone={toast.tone} onDismiss={dismissToast} />}

      {actionError && (
        <p className="form-error" role="alert">
          {actionError}
        </p>
      )}

      {loadError ? (
        <div className="state" role="alert">
          <p>
            {es.catalog.loadError} {loadError}
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
          <h2>{hasFilters ? es.catalog.noResultsTitle : es.catalog.emptyTitle}</h2>
          <p className="muted">{hasFilters ? es.catalog.noResultsBody : es.catalog.emptyBody}</p>
          {!hasFilters && (
            <Link to="/catalogo/nuevo" className="button button-primary">
                  {es.catalog.newProduct}
            </Link>
          )}
        </div>
      ) : (
        <>
          <div className="table-wrap" aria-busy={loading}>
            <table className="table-cards">
              <thead>
                <tr>
                  <th scope="col" className="select-cell">
                    <input
                      type="checkbox"
                      aria-label={es.catalog.labels.selectPage}
                      checked={data.items.every((item) => selected.has(item.id))}
                      onChange={(event) => togglePage(data.items, event.target.checked)}
                    />
                  </th>
                  <th scope="col">{es.catalog.columns.image}</th>
                  <th scope="col">{es.catalog.columns.sku}</th>
                  <th scope="col">{es.catalog.columns.name}</th>
                  <th scope="col">{es.catalog.columns.type}</th>
                  <th scope="col">{es.catalog.columns.price}</th>
                  <th scope="col">{es.catalog.columns.status}</th>
                  <th scope="col">{es.catalog.columns.actions}</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((product) => (
                  <ProductRow
                    key={product.id}
                    product={product}
                    confirming={confirmingId === product.id}
                    confirmingReset={resettingId === product.id}
                    onAskReset={() => setResettingId(product.id)}
                    onCancelReset={() => setResettingId(null)}
                    onReset={() => resetCode(product)}
                    busy={busyId === product.id}
                    selected={selected.has(product.id)}
                    onSelect={(on) => toggleSelected(product.id, on)}
                    onAskDeactivate={() => setConfirmingId(product.id)}
                    onCancelDeactivate={() => setConfirmingId(null)}
                    onDeactivate={() => changeActive(product, false)}
                    onActivate={() => changeActive(product, true)}
                  />
                ))}
              </tbody>
            </table>
          </div>

          <footer className="pagination">
            <p className="muted" aria-live="polite">
              {es.catalog.resultsSummary(data.items.length, data.total)}
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

interface ProductRowProps {
  product: Product
  confirming: boolean
  confirmingReset: boolean
  onAskReset: () => void
  onCancelReset: () => void
  onReset: () => void
  busy: boolean
  selected: boolean
  onSelect: (on: boolean) => void
  onAskDeactivate: () => void
  onCancelDeactivate: () => void
  onDeactivate: () => void
  onActivate: () => void
}

function ProductRow({
  product,
  confirming,
  confirmingReset,
  onAskReset,
  onCancelReset,
  onReset,
  busy,
  selected,
  onSelect,
  onAskDeactivate,
  onCancelDeactivate,
  onDeactivate,
  onActivate,
}: ProductRowProps) {
  return (
    <tr className={product.is_active ? undefined : 'is-inactive'}>
      <td className="select-cell">
        <input
          type="checkbox"
          aria-label={es.catalog.labels.selectRow(product.name)}
          checked={selected}
          onChange={(event) => onSelect(event.target.checked)}
        />
      </td>
      <td data-label={es.catalog.columns.image}>
        {product.image_url ? (
          <img className="thumbnail" src={product.image_url} alt={es.catalog.imageAlt(product.name)} loading="lazy" />
        ) : (
          <span className="thumbnail thumbnail-empty">{es.catalog.noImage}</span>
        )}
      </td>
      <td data-label={es.catalog.columns.sku} className="mono">{product.sku}</td>
      <td data-label={es.catalog.columns.name}>{product.name}</td>
      <td data-label={es.catalog.columns.type}>{es.catalog.types[product.product_type]}</td>
      <td data-label={es.catalog.columns.price}>
        <ReferencePrices product={product} />
      </td>
      <td data-label={es.catalog.columns.status}>
        <span className={product.is_active ? 'badge badge-active' : 'badge badge-inactive'}>
          {product.is_active ? es.catalog.status.active : es.catalog.status.inactive}
        </span>
      </td>
      <td data-label={es.catalog.columns.actions}>
        {confirming ? (
          <div className="confirm" role="group" aria-label={es.catalog.confirmDeactivate(product.name)}>
            <p>{es.catalog.confirmDeactivate(product.name)}</p>
            <button type="button" className="button button-danger" disabled={busy} onClick={onDeactivate}>
              {es.catalog.confirmDeactivateYes}
            </button>
            <button type="button" className="button" disabled={busy} onClick={onCancelDeactivate}>
              {es.common.cancel}
            </button>
          </div>
        ) : confirmingReset ? (
          <div className="confirm" role="group" aria-label={es.catalog.labels.confirmReset(product.name)}>
            <p>{es.catalog.labels.confirmReset(product.name)}</p>
            <button type="button" className="button button-danger" disabled={busy} onClick={onReset}>
              {es.catalog.labels.confirmResetYes}
            </button>
            <button type="button" className="button" disabled={busy} onClick={onCancelReset}>
              {es.common.cancel}
            </button>
          </div>
        ) : (
          <div className="row-actions">
            <IconLink
              to={`/catalogo/${product.id}/editar`}
              label={`${es.common.edit} ${product.name}`}
              icon={<PencilIcon />}
            />
            <IconButton
              label={`${es.catalog.labels.reset} ${product.name}`}
              icon={<RefreshIcon />}
              disabled={busy}
              onClick={onAskReset}
            />
            {product.is_active ? (
              <IconButton
                label={`${es.catalog.deactivate} ${product.name}`}
                icon={<EyeOffIcon />}
                disabled={busy}
                onClick={onAskDeactivate}
              />
            ) : (
              <IconButton
                label={`${es.catalog.activate} ${product.name}`}
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

function ReferencePrices({ product }: { product: Product }) {
  if (product.pvc) {
    return (
      <>
        <div>{es.catalog.pricePerMeter(formatDecimalEs(product.pvc.reference_price_per_meter))}</div>
        <div>{es.catalog.pricePerRoll(formatDecimalEs(product.pvc.reference_price_per_roll))}</div>
      </>
    )
  }
  if (product.tablecloth) {
    return <div>{es.catalog.pricePerUnit(formatDecimalEs(product.tablecloth.unit_price))}</div>
  }
  return null
}
