import { useEffect, useRef, useState, type ChangeEvent, type DragEvent } from 'react'
import { ImageIcon, ImagePlusIcon, TrashIcon } from '../../components/icons'
import { IconButton } from '../../components/IconButton'
import { es } from '../../i18n/es'
import { ACCEPTED_IMAGE_TYPES, validateImageFile } from './imageRules'

interface ImagePickerProps {
  /** Called with the chosen file, or null when the choice is removed. */
  onChange: (file: File | null) => void
  disabled?: boolean
}

/** Lets the user choose a reference image before the product exists; it is uploaded on save. */
export function ImagePicker({ onChange, disabled = false }: ImagePickerProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [selected, setSelected] = useState<{ name: string; previewUrl: string } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [dragging, setDragging] = useState(false)

  // Preview URLs are created in the event handler and released when replaced, removed or unmounted.
  const previewUrlRef = useRef<string | null>(null)
  useEffect(
    () => () => {
      if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current)
    },
    [],
  )

  function replaceSelection(file: File | null) {
    if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current)
    previewUrlRef.current = file ? URL.createObjectURL(file) : null
    setSelected(file && previewUrlRef.current ? { name: file.name, previewUrl: previewUrlRef.current } : null)
    onChange(file)
  }

  function acceptFile(file: File) {
    const problem = validateImageFile(file)
    if (problem) {
      setError(problem)
      return
    }
    setError(null)
    replaceSelection(file)
  }

  function handleFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (file) acceptFile(file)
  }

  function handleDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault()
    setDragging(false)
    const file = event.dataTransfer.files?.[0]
    if (file && !disabled) acceptFile(file)
  }

  const openFileDialog = () => {
    if (!disabled) inputRef.current?.click()
  }

  return (
    <div className="image-picker">
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPTED_IMAGE_TYPES.join(',')}
        className="visually-hidden"
        aria-label={es.image.fileInput}
        onChange={handleFile}
        tabIndex={-1}
      />

      {selected ? (
        <div className="dropzone has-image" onDragOver={(event) => event.preventDefault()} onDrop={handleDrop}>
          <img className="image-preview" src={selected.previewUrl} alt={es.image.previewAlt} />
          <div className="dropzone-text">
            <p className="dropzone-title">{es.image.selectedFile(selected.name)}</p>
            <p className="dropzone-hint">{es.image.optionalHint}</p>
          </div>
          <div className="form-actions">
            <IconButton label={es.image.change} icon={<ImageIcon />} disabled={disabled} onClick={openFileDialog} />
            <IconButton
              label={es.image.remove}
              icon={<TrashIcon />}
              variant="danger"
              disabled={disabled}
              onClick={() => {
                setError(null)
                replaceSelection(null)
              }}
            />
          </div>
        </div>
      ) : (
        <div
          className="dropzone"
          data-active={dragging || undefined}
          onClick={openFileDialog}
          onDragOver={(event) => {
            event.preventDefault()
            setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={handleDrop}
        >
          <span className="dropzone-tile" aria-hidden="true">
            <ImagePlusIcon />
          </span>
          <div className="dropzone-text">
            <p className="dropzone-title">{es.image.noneSelected}</p>
            <p className="dropzone-hint">{dragging ? es.image.dropActive : es.image.dropHint}</p>
            <p className="field-hint">{es.image.optionalHint}</p>
          </div>
          <button type="button" className="button" disabled={disabled}>
            <ImageIcon />
            {es.image.choose}
          </button>
        </div>
      )}

      {error && (
        <p className="field-error" role="alert">
          {error}
        </p>
      )}
    </div>
  )
}
