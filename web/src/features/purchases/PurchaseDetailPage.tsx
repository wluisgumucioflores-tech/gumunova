import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { isAbortError, messageForApiError } from '../../api/client'
import { getPurchase, type Purchase, type PurchaseLine } from '../../api/purchases'
import { IconLink } from '../../components/IconButton'
import { PencilIcon } from '../../components/icons'
import { es } from '../../i18n/es'
import { formatDateEs, formatDecimalEs } from '../../lib/decimal'

const t = es.purchases.detail
const money = (amount: string) => es.purchases.money(formatDecimalEs(amount))

export function PurchaseDetailPage() {
  const { id = '' } = useParams()
  const [reloadToken, setReloadToken] = useState(0)

  // The loaded purchase (or error), tagged with the request that produced it, so "loading" is
  // derived rather than set from inside the effect.
  const [loaded, setLoaded] = useState<{ key: string; purchase: Purchase | null; error: string | null } | null>(null)
  const requestKey = `${id}#${reloadToken}`
  const current = loaded?.key === requestKey ? loaded : null
  const purchase = current?.purchase ?? null
  const loadError = current?.error ?? null

  useEffect(() => {
    const controller = new AbortController()
    getPurchase(id, controller.signal)
      .then((result) => setLoaded({ key: requestKey, purchase: result, error: null }))
      .catch((error: unknown) => {
        if (!isAbortError(error)) setLoaded({ key: requestKey, purchase: null, error: messageForApiError(error) })
      })
    return () => controller.abort()
  }, [id, requestKey])

  return (
    <>
      <Link to="/compras" className="back-link">
        {es.purchases.back}
      </Link>
      <header className="page-header">
        <h1>{t.title}</h1>
        {purchase && <IconLink to={`/compras/${purchase.id}/editar`} label={es.purchases.edit} icon={<PencilIcon />} />}
      </header>

      {loadError ? (
        <div className="state" role="alert">
          <p>
            {t.loadError} {loadError}
          </p>
          <button type="button" className="button" onClick={() => setReloadToken((token) => token + 1)}>
            {es.common.retry}
          </button>
        </div>
      ) : purchase === null ? (
        <p className="state" role="status">
          {es.common.loading}
        </p>
      ) : (
        <PurchaseView purchase={purchase} />
      )}
    </>
  )
}

function PurchaseView({ purchase }: { purchase: Purchase }) {
  return (
    <div className="detail">
      <section className="panel" aria-label={t.title}>
        <dl className="facts">
          <div>
            <dt>{t.supplier}</dt>
            <dd>{purchase.supplier?.name ?? es.purchases.noSupplier}</dd>
          </div>
          <div>
            <dt>{t.date}</dt>
            <dd>{formatDateEs(purchase.purchase_date)}</dd>
          </div>
          <div>
            <dt>{t.reference}</dt>
            <dd>{purchase.document_reference || es.common.noValue}</dd>
          </div>
          <div>
            <dt>{t.statusLabel}</dt>
            <dd>
              <span className="badge badge-active">{t.status[purchase.status]}</span>
            </dd>
          </div>
          <div>
            <dt>{t.additionalCosts}</dt>
            <dd>{money(purchase.additional_costs)}</dd>
          </div>
          <div className="summary-total">
            <dt>{t.total}</dt>
            <dd>{money(purchase.total_amount)}</dd>
          </div>
        </dl>
        {purchase.notes && (
          <p>
            <strong>{t.notes}:</strong> {purchase.notes}
          </p>
        )}
        <p className="field-hint">{purchase.items_editable ? t.editableNote : t.lockedNote}</p>
        {purchase.revision > 1 && <p className="field-hint">{t.revisionNote(purchase.revision)}</p>}
      </section>

      {purchase.lines.map((line) => (
        <LineView key={line.id} line={line} />
      ))}
    </div>
  )
}

function LineView({ line }: { line: PurchaseLine }) {
  return (
    <section className="panel" aria-label={t.lineHeading(line.line_number, line.product.name)}>
      <h2>{t.lineHeading(line.line_number, line.product.name)}</h2>
      <p className="muted">
        {t.sku}: <span className="mono">{line.product.sku}</span> ·{' '}
        {es.catalog.types[line.product.product_type]}
      </p>

      {line.rolls.length > 0 ? (
        <>
          <h3>{t.rollsCreated}</h3>
          <div className="table-wrap">
            <table className="table-cards">
              <thead>
                <tr>
                  <th scope="col">{t.rollCode}</th>
                  <th scope="col" className="numeric">
                    {t.rollLength}
                  </th>
                  <th scope="col" className="numeric">
                    {t.rollCost}
                  </th>
                  <th scope="col" className="numeric">
                    {t.rollAdditional}
                  </th>
                  <th scope="col" className="numeric">
                    {t.rollAttributable}
                  </th>
                </tr>
              </thead>
              <tbody>
                {line.rolls.map((roll) => (
                  <tr key={roll.id}>
                    <td data-label={t.rollCode} className="mono">{roll.roll_code}</td>
                    <td data-label={t.rollLength} className="numeric">{formatDecimalEs(roll.length_m)} m</td>
                    <td data-label={t.rollCost} className="numeric">{money(roll.acquisition_cost)}</td>
                    <td data-label={t.rollAdditional} className="numeric">{money(roll.additional_cost)}</td>
                    <td data-label={t.rollAttributable} className="numeric">{money(roll.attributable_cost)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      ) : (
        <dl className="facts">
          <div>
            <dt>{t.quantity}</dt>
            <dd>{line.quantity}</dd>
          </div>
          <div>
            <dt>{t.unitCost}</dt>
            <dd>{line.unit_cost ? money(line.unit_cost) : es.common.noValue}</dd>
          </div>
          <div>
            <dt>{t.additionalShare}</dt>
            <dd>{money(line.additional_cost)}</dd>
          </div>
          <div>
            <dt>{t.lineTotal}</dt>
            <dd>{money(line.line_total)}</dd>
          </div>
        </dl>
      )}
      {line.rolls.length === 0 && <p className="field-hint">{t.noRollsNote}</p>}
    </section>
  )
}
