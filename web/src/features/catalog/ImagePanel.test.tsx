import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as api from '../../api/products'
import { es } from '../../i18n/es'
import { pvcProduct } from '../../test/fixtures'
import { ImagePanel } from './ImagePanel'

vi.mock('../../api/products')

const uploadProductImage = vi.mocked(api.uploadProductImage)
const removeProductImage = vi.mocked(api.removeProductImage)

const withImage = () => pvcProduct({ image_url: 'https://storage.test/a.png?signed=1' })

function file(name: string, type: string, size = 10) {
  return new File([new Uint8Array(size)], name, { type })
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('ImagePanel', () => {
  it('says there is no image and offers to upload one', () => {
    render(<ImagePanel product={pvcProduct()} onChange={vi.fn()} />)

    expect(screen.getByText(es.image.none)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: es.image.upload })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: es.image.remove })).not.toBeInTheDocument()
  })

  it('shows the current image with an accessible description', () => {
    render(<ImagePanel product={withImage()} onChange={vi.fn()} />)

    expect(screen.getByRole('img', { name: es.catalog.imageAlt('PVC floral 20 m') })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: es.image.change })).toBeInTheDocument()
  })

  it('uploads a valid image and reports the updated product', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    uploadProductImage.mockResolvedValue(withImage())
    render(<ImagePanel product={pvcProduct()} onChange={onChange} />)

    await user.upload(screen.getByLabelText(es.image.fileInput), file('foto.png', 'image/png'))

    await waitFor(() => expect(onChange).toHaveBeenCalledWith(withImage()))
    expect(uploadProductImage).toHaveBeenCalledWith('pvc-1', expect.any(File))
    expect(screen.getByRole('status')).toHaveTextContent(es.image.uploaded)
  })

  it('rejects unsupported types without calling the API', async () => {
    const user = userEvent.setup({ applyAccept: false })
    render(<ImagePanel product={pvcProduct()} onChange={vi.fn()} />)

    await user.upload(screen.getByLabelText(es.image.fileInput), file('nota.gif', 'image/gif'))

    expect(screen.getByRole('alert')).toHaveTextContent(es.image.typeClient)
    expect(uploadProductImage).not.toHaveBeenCalled()
  })

  it('rejects images over 5 MB without calling the API', async () => {
    const user = userEvent.setup()
    render(<ImagePanel product={pvcProduct()} onChange={vi.fn()} />)

    await user.upload(screen.getByLabelText(es.image.fileInput), file('grande.png', 'image/png', 5 * 1024 * 1024 + 1))

    expect(screen.getByRole('alert')).toHaveTextContent(es.image.tooLargeClient)
    expect(uploadProductImage).not.toHaveBeenCalled()
  })

  it('shows the server error in Spanish when the upload fails', async () => {
    const user = userEvent.setup()
    uploadProductImage.mockRejectedValue(new ApiError(415, 'image_type_not_supported'))
    render(<ImagePanel product={pvcProduct()} onChange={vi.fn()} />)

    await user.upload(screen.getByLabelText(es.image.fileInput), file('foto.png', 'image/png'))

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.image_type_not_supported)
  })

  it('removes the image', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    removeProductImage.mockResolvedValue(pvcProduct())
    render(<ImagePanel product={withImage()} onChange={onChange} />)

    await user.click(screen.getByRole('button', { name: es.image.remove }))

    await waitFor(() => expect(onChange).toHaveBeenCalledWith(pvcProduct()))
    expect(removeProductImage).toHaveBeenCalledWith('pvc-1')
    expect(screen.getByRole('status')).toHaveTextContent(es.image.removed)
  })
})
