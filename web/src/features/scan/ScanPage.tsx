import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from 'react'
import { useParams } from 'react-router'
import { ApiError, isAbortError, messageForApiError } from '../../api/client'
import { lookupLabel, type LabelLookup } from '../../api/labels'
import type { Product } from '../../api/products'
import { CameraScanner } from '../../components/CameraScanner'
import { es } from '../../i18n/es'
import { formatDecimalEs } from '../../lib/decimal'
import { codeFromScan } from '../../lib/labelLink'
import { cameraAvailable, readCodeFromImageFile } from '../../lib/scanner'

const t = es.scan

type Outcome =
  | { state: 'idle' }
  | { state: 'looking' }
  | { state: 'found'; lookup: LabelLookup }
  | { state: 'unknown' }
  | { state: 'failed'; message: string }

export function ScanPage() {
  // A label's QR opens /producto/<code>: look the code of the address up right away.
  const addressCode = useParams().code?.trim() ?? ''
  const [outcome, setOutcome] = useState<Outcome>(addressCode ? { state: 'looking' } : { state: 'idle' })
  const [cameraOn, setCameraOn] = useState(false)
  const [cameraProblem, setCameraProblem] = useState(false)
  const [manualCode, setManualCode] = useState('')
  const [photoProblem, setPhotoProblem] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const controllerRef = useRef<AbortController | null>(null)

  // A hardware scanner types the code into the focused field and presses Enter.
  useEffect(() => inputRef.current?.focus(), [])
  useEffect(() => () => controllerRef.current?.abort(), [])

  useEffect(() => {
    if (addressCode) void lookup(addressCode)
    // Only when the address brings a (different) code; `lookup` is recreated on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [addressCode])

  async function lookup(scanned: string) {
    // What a QR carries is an address; a barcode or the keyboard gives the bare code.
    const code = codeFromScan(scanned)
    controllerRef.current?.abort()
    const controller = new AbortController()
    controllerRef.current = controller
    setCameraOn(false)
    setPhotoProblem(null)
    setOutcome({ state: 'looking' })
    try {
      const result = await lookupLabel(code, controller.signal)
      setOutcome({ state: 'found', lookup: result })
    } catch (error) {
      if (isAbortError(error)) return
      if (error instanceof ApiError && error.code === 'label_not_found') setOutcome({ state: 'unknown' })
      else setOutcome({ state: 'failed', message: messageForApiError(error) })
    }
  }

  function submitManual(event: FormEvent) {
    event.preventDefault()
    const code = manualCode.trim()
    if (!code) return
    setManualCode('')
    void lookup(code)
  }

  async function readPhoto(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = '' // so the same photo can be chosen again
    if (!file) return
    setPhotoProblem(null)
    try {
      const code = await readCodeFromImageFile(file)
      if (code) void lookup(code)
      else setPhotoProblem(t.noCodeInPhoto)
    } catch {
      setPhotoProblem(t.noCodeInPhoto)
    }
  }

  function scanAnother() {
    setOutcome({ state: 'idle' })
    setPhotoProblem(null)
    inputRef.current?.focus()
  }

  const canUseCamera = cameraAvailable()

  return (
    <>
      <header className="page-header">
        <div>
          <h1>{t.title}</h1>
          <p className="muted">{t.subtitle}</p>
        </div>
      </header>

      <section className="panel scan-panel" aria-label={t.title}>
        {cameraOn ? (
          <>
            <CameraScanner
              onCode={(text) => void lookup(text)}
              onError={() => {
                setCameraOn(false)
                setCameraProblem(true)
              }}
            />
            <p className="field-hint">{t.pointAtCode}</p>
            <button type="button" className="button" onClick={() => setCameraOn(false)}>
              {t.stopCamera}
            </button>
          </>
        ) : (
          <div className="form-actions">
            {canUseCamera && (
              <button
                type="button"
                className="button button-primary"
                onClick={() => {
                  setCameraProblem(false)
                  setOutcome({ state: 'idle' })
                  setCameraOn(true)
                }}
              >
                {t.useCamera}
              </button>
            )}
            <label className="button">
              {t.takePhoto}
              <input type="file" accept="image/*" capture="environment" className="visually-hidden" onChange={readPhoto} />
            </label>
          </div>
        )}

        {!canUseCamera && <p className="field-hint">{t.cameraUnavailable}</p>}
        {cameraProblem && (
          <p className="form-error" role="alert">
            {t.cameraFailed}
          </p>
        )}
        {photoProblem && (
          <p className="form-error" role="alert">
            {photoProblem}
          </p>
        )}

        <form className="scan-manual" onSubmit={submitManual}>
          <div className="field">
            <label htmlFor="scan-code">{t.manualLabel}</label>
            <input
              id="scan-code"
              ref={inputRef}
              value={manualCode}
              autoComplete="off"
              autoCapitalize="characters"
              spellCheck={false}
              placeholder={t.manualPlaceholder}
              onChange={(event) => setManualCode(event.target.value)}
            />
            <p className="field-hint">{t.manualHint}</p>
          </div>
          <button type="submit" className="button" disabled={manualCode.trim() === ''}>
            {t.search}
          </button>
        </form>
      </section>

      <div aria-live="polite">
        {outcome.state === 'looking' && (
          <p className="state" role="status">
            {es.common.loading}
          </p>
        )}
        {outcome.state === 'unknown' && (
          <div className="state" role="alert">
            <h2>{t.unknownTitle}</h2>
            <p className="muted">{t.unknownBody}</p>
            <button type="button" className="button" onClick={scanAnother}>
              {t.scanAnother}
            </button>
          </div>
        )}
        {outcome.state === 'failed' && (
          <div className="state" role="alert">
            <p>
              {t.lookupError} {outcome.message}
            </p>
            <button type="button" className="button" onClick={scanAnother}>
              {t.scanAnother}
            </button>
          </div>
        )}
        {outcome.state === 'found' && <ScanResult lookup={outcome.lookup} onScanAnother={scanAnother} />}
      </div>
    </>
  )
}

function ScanResult({ lookup, onScanAnother }: { lookup: LabelLookup; onScanAnother: () => void }) {
  const { product, roll } = lookup
  return (
    <section className="panel scan-result" aria-label={t.resultLabel}>
      <div className="scan-result-head">
        {product.image_url ? (
          <img className="thumbnail" src={product.image_url} alt={es.catalog.imageAlt(product.name)} />
        ) : (
          <span className="thumbnail thumbnail-empty">{es.catalog.noImage}</span>
        )}
        <div>
          <h2>{product.name}</h2>
          <p className="muted">
            {es.catalog.types[product.product_type]} · <span className="mono">{product.sku}</span>
            {!product.is_active && <span className="badge badge-inactive">{es.catalog.status.inactive}</span>}
          </p>
        </div>
      </div>

      <h3>{t.prices}</h3>
      <Prices product={product} />

      {roll && (
        <>
          <h3>{t.rollHeading(roll.code)}</h3>
          <dl className="facts">
            <div>
              <dt>{t.available}</dt>
              <dd>{es.inventory.meters(formatDecimalEs(roll.available_length_m))}</dd>
            </div>
            <div>
              <dt>{t.initial}</dt>
              <dd>{es.inventory.meters(formatDecimalEs(roll.initial_length_m))}</dd>
            </div>
          </dl>
          <p className="field-hint">{roll.is_intact ? t.rollIntact : t.rollCutHint}</p>
        </>
      )}

      <div className="form-actions">
        <button type="button" className="button button-primary" onClick={onScanAnother}>
          {t.scanAnother}
        </button>
      </div>
    </section>
  )
}

/** The reference prices that apply to the kind of product: per meter and per roll for PVC, per unit for tablecloths. */
function Prices({ product }: { product: Product }) {
  if (product.pvc) {
    return (
      <dl className="facts scan-prices">
        <div>
          <dt>{t.perMeter}</dt>
          <dd className="scan-price">{es.purchases.money(formatDecimalEs(product.pvc.reference_price_per_meter))}</dd>
        </div>
        <div>
          <dt>{t.perRoll}</dt>
          <dd className="scan-price">{es.purchases.money(formatDecimalEs(product.pvc.reference_price_per_roll))}</dd>
        </div>
      </dl>
    )
  }
  if (product.tablecloth) {
    return (
      <dl className="facts scan-prices">
        <div>
          <dt>{t.perUnit}</dt>
          <dd className="scan-price">{es.purchases.money(formatDecimalEs(product.tablecloth.unit_price))}</dd>
        </div>
      </dl>
    )
  }
  return null
}
