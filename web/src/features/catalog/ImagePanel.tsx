import { useRef, useState, type ChangeEvent } from 'react'
import { messageForApiError } from '../../api/client'
import { removeProductImage, uploadProductImage, type Product } from '../../api/products'
import { TrashIcon, UploadIcon } from '../../components/icons'
import { IconButton } from '../../components/IconButton'
import { es } from '../../i18n/es'
import { ACCEPTED_IMAGE_TYPES, validateImageFile } from './imageRules'

interface ImagePanelProps {
  product: Product
  onChange: (product: Product) => void
}

type Busy = 'uploading' | 'removing' | null

export function ImagePanel({ product, onChange }: ImagePanelProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState<Busy>(null)
  const [message, setMessage] = useState<{ kind: 'error' | 'success'; text: string } | null>(null)

  async function handleFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file || busy) return

    const problem = validateImageFile(file)
    if (problem) {
      setMessage({ kind: 'error', text: problem })
      return
    }

    setBusy('uploading')
    setMessage(null)
    try {
      onChange(await uploadProductImage(product.id, file))
      setMessage({ kind: 'success', text: es.image.uploaded })
    } catch (error) {
      setMessage({ kind: 'error', text: messageForApiError(error) })
    } finally {
      setBusy(null)
    }
  }

  async function handleRemove() {
    if (busy) return
    setBusy('removing')
    setMessage(null)
    try {
      onChange(await removeProductImage(product.id))
      setMessage({ kind: 'success', text: es.image.removed })
    } catch (error) {
      setMessage({ kind: 'error', text: messageForApiError(error) })
    } finally {
      setBusy(null)
    }
  }

  return (
    <section className="panel" aria-labelledby="image-panel-title">
      <h2 id="image-panel-title">{es.form.sections.image}</h2>

      {product.image_url ? (
        <img className="image-preview" src={product.image_url} alt={es.catalog.imageAlt(product.name)} />
      ) : (
        <p className="muted">{es.image.none}</p>
      )}

      <div className="form-actions">
        <input
          ref={inputRef}
          type="file"
          accept={ACCEPTED_IMAGE_TYPES.join(',')}
          className="visually-hidden"
          aria-label={es.image.fileInput}
          onChange={handleFile}
          tabIndex={-1}
        />
        <IconButton
          label={busy === 'uploading' ? es.image.uploading : product.image_url ? es.image.change : es.image.upload}
          icon={<UploadIcon />}
          disabled={busy !== null}
          onClick={() => inputRef.current?.click()}
        />
        {product.image_url && (
          <IconButton
            label={busy === 'removing' ? es.image.removing : es.image.remove}
            icon={<TrashIcon />}
            variant="danger"
            disabled={busy !== null}
            onClick={handleRemove}
          />
        )}
      </div>
      <p className="field-hint">{es.image.hint}</p>

      {message && (
        <p className={message.kind === 'error' ? 'form-error' : 'form-success'} role={message.kind === 'error' ? 'alert' : 'status'}>
          {message.text}
        </p>
      )}
    </section>
  )
}
