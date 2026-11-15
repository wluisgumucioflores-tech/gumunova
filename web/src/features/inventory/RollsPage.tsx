import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { messageForApiError } from '../../api/client'
import { listRolls, resetRollLabelCode, type Roll, type RollStatus } from '../../api/inventory'
import { ListIcon, ReceiptIcon, RefreshIcon } from '../../components/icons'
import { IconButton, IconLink } from '../../components/IconButton'
import { Toast } from '../../components/Toast'
import { useNavigationToast } from '../../hooks/useNavigationToast'
import { useRemoteList } from '../../hooks/useRemoteList'
import { es } from '../../i18n/es'
import { formatDateEs, formatDecimalEs } from '../../lib/decimal'
import { Pagination } from './Pagination'

const PAGE_SIZE = 25
const SEARCH_DELAY_MS = 300
const t = es.inventory
const r = t.rolls

function statusBadge(roll: Roll) {
  if (Number(roll.available_length_m.replace('.', '')) === 0) return <span className="badge badge-inactive">{r.depleted}</span>
  return roll.is_intact ? <span className="badge badge-active">{r.intact}</span> : <span className="badge badge-warning">{r.cut}</span>
}

export function RollsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const productId = searchParams.get('producto') ?? ''

  const [status, setStatus] = useState<RollStatus>('available')
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

  const { toast, dismissToast, showToast } = useNavigationToast()
  // Rolls ticked for printing labels; the selection survives changing page and filters.
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set())
  const [resettingId, setResettingId] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  function setSelection(rolls: Roll[], on: boolean) {
    setSelected((current) => {
      const next = new Set(current)
      for (const roll of rolls) {
        if (on) next.add(roll.id)
        else next.delete(roll.id)
      }
      return next
    })
  }

  async function resetCode(roll: Roll) {
    setBusyId(roll.id)
    setActionError(null)
    try {
      await resetRollLabelCode(roll.id)
      setResettingId(null)
      showToast(r.labels.resetDone(roll.code))
    } catch (failure) {
      setActionError(`${r.labels.resetError} ${messageForApiError(failure)}`)
    } finally {
      setBusyId(null)
    }
  }

  const { data, loading, error, reload } = useRemoteList(JSON.stringify([productId, status, query, offset]), (signal) =>
    listRolls({ productId: productId || undefined, status, query, limit: PAGE_SIZE, offset }, signal),
  )
  const hasFilters = productId !== '' || status !== 'available' || query !== ''

  return (
    <>
      <div className="inline-actions">
        <Link to="/inventario/rollos/etiquetas?todos=1" className="button">
          {r.labels.openAll}
        </Link>
      </div>

      {selected.size > 0 && (
        <div className="selection-bar" role="group" aria-label={r.labels.selectedCount(selected.size)}>
          <span>{r.labels.selectedCount(selected.size)}</span>
          <Link to={`/inventario/rollos/etiquetas?ids=${[...selected].join(',')}`} className="button button-primary">
            {r.labels.open}
          </Link>
          <button type="button" className="button" onClick={() => setSelected(new Set())}>
            {r.labels.clearSelection}
          </button>
        </div>
      )}

      {toast && <Toast text={toast.text} tone={toast.tone} onDismiss={dismissToast} />}
      {actionError && (
        <p className="form-error" role="alert">
          {actionError}
        </p>
      )}

      {productId && (
        <p className="notice">
          {r.filteredByProduct}{' '}
          <button
            type="button"
            className="link-button"
            onClick={() => {
              setSearchParams({})
              setOffset(0)
            }}
          >
            {r.clearProduct}
          </button>
        </p>
      )}

      <div className="filters filters-two">
        <div className="field">
          <label htmlFor="roll-search">{r.searchLabel}</label>
          <input id="roll-search" type="search" value={searchInput} onChange={(event) => setSearchInput(event.target.value)} />
        </div>
        <div className="field">
          <label htmlFor="roll-status">{r.statusLabel}</label>
          <select
            id="roll-status"
            value={status}
            onChange={(event) => {
              setStatus(event.target.value as RollStatus)
              setOffset(0)
            }}
          >
            <option value="available">{r.statuses.available}</option>
            <option value="depleted">{r.statuses.depleted}</option>
            <option value="all">{r.statuses.all}</option>
          </select>
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
          <h2>{hasFilters ? r.noResultsTitle : r.emptyTitle}</h2>
          <p className="muted">{hasFilters ? r.noResultsBody : r.emptyBody}</p>
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
                  <th scope="col" className="select-cell">
                    <input
                      type="checkbox"
                      aria-label={r.labels.selectPage}
                      checked={data.items.every((item) => selected.has(item.id))}
                      onChange={(event) => setSelection(data.items, event.target.checked)}
                    />
                  </th>
                  <th scope="col">{r.columns.code}</th>
                  <th scope="col">{r.columns.product}</th>
                  <th scope="col">{r.columns.received}</th>
                  <th scope="col" className="numeric">
                    {r.columns.initial}
                  </th>
                  <th scope="col" className="numeric">
                    {r.columns.available}
                  </th>
                  <th scope="col" className="numeric">
                    {r.columns.cost}
                  </th>
                  <th scope="col" className="numeric">
                    {r.columns.costPerMeter}
                  </th>
                  <th scope="col" className="numeric">
                    {r.columns.remaining}
                  </th>
                  <th scope="col">{r.columns.status}</th>
                  <th scope="col">{r.columns.actions}</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((roll) => (
                  <tr key={roll.id}>
                    <td className="select-cell">
                      <input
                        type="checkbox"
                        aria-label={r.labels.selectRow(roll.code)}
                        checked={selected.has(roll.id)}
                        onChange={(event) => setSelection([roll], event.target.checked)}
                      />
                    </td>
                    <td data-label={r.columns.code}>{roll.code}</td>
                    <td data-label={r.columns.product}>
                      {roll.product.sku} — {roll.product.name}
                    </td>
                    <td data-label={r.columns.received}>{formatDateEs(roll.received_at)}</td>
                    <td data-label={r.columns.initial} className="numeric">{t.meters(formatDecimalEs(roll.initial_length_m))}</td>
                    <td data-label={r.columns.available} className="numeric">{t.meters(formatDecimalEs(roll.available_length_m))}</td>
                    <td data-label={r.columns.cost} className="numeric">{t.money(formatDecimalEs(roll.attributable_cost))}</td>
                    <td data-label={r.columns.costPerMeter} className="numeric">{t.money(formatDecimalEs(roll.cost_per_meter))}</td>
                    <td data-label={r.columns.remaining} className="numeric">{t.money(formatDecimalEs(roll.remaining_value))}</td>
                    <td data-label={r.columns.status}>{statusBadge(roll)}</td>
                    <td data-label={r.columns.actions}>
                      {resettingId === roll.id ? (
                        <div className="confirm" role="group" aria-label={r.labels.confirmReset(roll.code)}>
                          <p>{r.labels.confirmReset(roll.code)}</p>
                          <button type="button" className="button button-danger" disabled={busyId === roll.id} onClick={() => resetCode(roll)}>
                            {r.labels.confirmResetYes}
                          </button>
                          <button type="button" className="button" disabled={busyId === roll.id} onClick={() => setResettingId(null)}>
                            {es.common.cancel}
                          </button>
                        </div>
                      ) : (
                        <div className="row-actions">
                          <IconLink to={`/inventario/movimientos?rollo=${roll.id}`} label={`${r.viewMovements} ${roll.code}`} icon={<ListIcon />} />
                          <IconLink to={`/compras/${roll.purchase_id}`} label={`${r.viewPurchase} ${roll.code}`} icon={<ReceiptIcon />} />
                          <IconButton label={`${r.labels.reset} ${roll.code}`} icon={<RefreshIcon />} onClick={() => setResettingId(roll.id)} />
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="field-hint">{r.costHint}</p>
          <Pagination
            summary={r.resultsSummary(data.items.length, data.total)}
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
