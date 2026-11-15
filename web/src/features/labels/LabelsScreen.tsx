import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { isAbortError, messageForApiError } from '../../api/client'
import { Barcode } from '../../components/Barcode'
import { QrCode } from '../../components/QrCode'
import { isLocalAddress, labelUrl, publicBaseUrl } from '../../lib/labelLink'
import { es } from '../../i18n/es'

const t = es.labels
const MAX_COPIES = 50

type CodeKind = 'qr' | 'barcode'

/** What a label shows: the unique code it carries, and two lines of text for people. */
export interface LabelEntry {
  /** The unique code the QR or barcode carries. */
  code: string
  primary: string
  secondary: string
}

export interface LabelsScreenStrings {
  title: string
  subtitle: string
  back: string
  summary: (items: number, labels: number) => string
  nothingSelected: string
  someMissing: (count: number) => string
  loadError: string
}

interface LabelsScreenProps<T extends { id: string }> {
  strings: LabelsScreenStrings
  /** Where "back" goes. */
  backTo: string
  /** Loads the candidates: only the active ones when `everything` is true, otherwise all of them. Keep it stable (declare it outside the component). */
  loadItems: (everything: boolean, signal: AbortSignal) => Promise<T[]>
  entryOf: (item: T) => LabelEntry
}

/**
 * Chooses and prints labels. Which items to print comes from the address: `ids=a,b` for a
 * selection, `todos=1` for everything available. It is shared by the product and roll labels.
 */
export function LabelsScreen<T extends { id: string }>({ strings, backTo, loadItems, entryOf }: LabelsScreenProps<T>) {
  const [params] = useSearchParams()
  const everything = params.get('todos') === '1'
  const idsText = params.get('ids') ?? ''
  const ids = idsText.split(',').map((id) => id.trim()).filter(Boolean)
  const nothingRequested = !everything && ids.length === 0

  const [reloadToken, setReloadToken] = useState(0)
  const [kind, setKind] = useState<CodeKind>('qr')
  const [copiesText, setCopiesText] = useState('1')

  // The loaded items, tagged with the request that produced them, so "loading" is derived.
  const requestKey = `${everything}#${reloadToken}`
  const [loaded, setLoaded] = useState<{ key: string; items: T[] | null; error: string | null } | null>(null)
  const current = loaded?.key === requestKey ? loaded : null

  useEffect(() => {
    if (nothingRequested) return // nothing to load: the page asks the person to choose
    const controller = new AbortController()
    loadItems(everything, controller.signal)
      .then((items) => setLoaded({ key: requestKey, items, error: null }))
      .catch((error: unknown) => {
        if (!isAbortError(error)) setLoaded({ key: requestKey, items: null, error: messageForApiError(error) })
      })
    return () => controller.abort()
  }, [everything, nothingRequested, requestKey, loadItems])

  const all = current?.items ?? null
  const byId = new Map((all ?? []).map((item) => [item.id, item]))
  const items = all === null ? [] : everything ? all : ids.map((id) => byId.get(id)).filter((item): item is T => item !== undefined)
  const missing = all !== null && !everything ? ids.length - items.length : 0

  const base = publicBaseUrl()
  const qrPointsToThisComputer = kind === 'qr' && isLocalAddress(base)

  const copies = /^\d{1,2}$/.test(copiesText.trim()) ? Math.min(Math.max(Number(copiesText), 1), MAX_COPIES) : 1
  const labels = items.flatMap((item) => Array.from({ length: copies }, (_, copy) => ({ item, copy })))

  return (
    <>
      <div className="no-print">
        <Link to={backTo} className="back-link">
          {strings.back}
        </Link>
        <header className="page-header">
          <div>
            <h1>{strings.title}</h1>
            <p className="muted">{strings.subtitle}</p>
          </div>
        </header>
      </div>

      {nothingRequested ? (
        <div className="state no-print">
          <p>{strings.nothingSelected}</p>
          <Link to={backTo} className="button">
            {strings.back}
          </Link>
        </div>
      ) : current?.error ? (
        <div className="state no-print" role="alert">
          <p>
            {strings.loadError} {current.error}
          </p>
          <button type="button" className="button" onClick={() => setReloadToken((token) => token + 1)}>
            {es.common.retry}
          </button>
        </div>
      ) : all === null ? (
        <p className="state no-print" role="status">
          {es.common.loading}
        </p>
      ) : (
        <>
          <div className="filters filters-three no-print">
            <div className="field">
              <label htmlFor="label-kind">{t.kindLabel}</label>
              <select id="label-kind" value={kind} onChange={(event) => setKind(event.target.value as CodeKind)}>
                <option value="qr">{t.kinds.qr}</option>
                <option value="barcode">{t.kinds.barcode}</option>
              </select>
            </div>
            <div className="field">
              <label htmlFor="label-copies">{t.copiesLabel}</label>
              <input
                id="label-copies"
                inputMode="numeric"
                value={copiesText}
                onChange={(event) => setCopiesText(event.target.value)}
                onBlur={() => setCopiesText(String(copies))}
              />
              <p className="field-hint">{t.copiesHint(MAX_COPIES)}</p>
            </div>
            <div className="field">
              <button type="button" className="button button-primary" disabled={labels.length === 0} onClick={() => window.print()}>
                {t.print}
              </button>
            </div>
          </div>

          {kind === 'qr' && (
            <p className={qrPointsToThisComputer ? 'notice no-print' : 'field-hint no-print'} role={qrPointsToThisComputer ? 'status' : undefined}>
              {qrPointsToThisComputer ? t.qrLocalWarning(base) : t.qrOpensPage(base)}
            </p>
          )}
          {missing > 0 && (
            <p className="notice no-print" role="status">
              {strings.someMissing(missing)}
            </p>
          )}
          {labels.length === 0 ? (
            <p className="state no-print">{t.noItems}</p>
          ) : (
            <>
              <p className="muted no-print" aria-live="polite">
                {strings.summary(items.length, labels.length)}
              </p>
              <ul className="label-sheet" aria-label={strings.title}>
                {labels.map(({ item, copy }) => {
                  const entry = entryOf(item)
                  return (
                    <li key={`${item.id}-${copy}`} className="label">
                      {kind === 'qr' ? (
                        <QrCode value={labelUrl(entry.code, base)} label={t.codeAlt(entry.primary)} />
                      ) : (
                        <Barcode value={entry.code} label={t.codeAlt(entry.primary)} />
                      )}
                      <span className="label-sku">{entry.primary}</span>
                      <span className="label-name">{entry.secondary}</span>
                      <span className="label-code">{entry.code}</span>
                    </li>
                  )
                })}
              </ul>
            </>
          )}
        </>
      )}
    </>
  )
}
