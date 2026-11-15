import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { es } from '../../i18n/es'
import { ImagePicker } from './ImagePicker'

function file(name: string, type: string, size = 10) {
  return new File([new Uint8Array(size)], name, { type })
}

beforeEach(() => {
  vi.mocked(URL.createObjectURL).mockReset().mockReturnValue('blob:preview')
  vi.mocked(URL.revokeObjectURL).mockReset()
})

describe('ImagePicker', () => {
  it('starts without a selection and says the image is optional', () => {
    render(<ImagePicker onChange={vi.fn()} />)

    expect(screen.getByText(es.image.noneSelected)).toBeInTheDocument()
    expect(screen.getByText(es.image.optionalHint)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: es.image.remove })).not.toBeInTheDocument()
  })

  it('shows a preview and the file name once an image is chosen', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<ImagePicker onChange={onChange} />)
    const chosen = file('floral.png', 'image/png')

    await user.upload(screen.getByLabelText(es.image.fileInput), chosen)

    expect(onChange).toHaveBeenCalledWith(chosen)
    expect(screen.getByRole('img', { name: es.image.previewAlt })).toHaveAttribute('src', 'blob:preview')
    expect(screen.getByText(es.image.selectedFile('floral.png'))).toBeInTheDocument()
    expect(screen.getByRole('button', { name: es.image.change })).toBeInTheDocument()
  })

  it('rejects unsupported types and files over 5 MB without selecting them', async () => {
    const user = userEvent.setup({ applyAccept: false })
    const onChange = vi.fn()
    render(<ImagePicker onChange={onChange} />)

    await user.upload(screen.getByLabelText(es.image.fileInput), file('nota.gif', 'image/gif'))
    expect(screen.getByRole('alert')).toHaveTextContent(es.image.typeClient)

    await user.upload(screen.getByLabelText(es.image.fileInput), file('grande.png', 'image/png', 5 * 1024 * 1024 + 1))
    expect(screen.getByRole('alert')).toHaveTextContent(es.image.tooLargeClient)

    expect(onChange).not.toHaveBeenCalled()
    expect(screen.getByText(es.image.noneSelected)).toBeInTheDocument()
  })

  it('removes the selection, tells the parent and releases the preview', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(<ImagePicker onChange={onChange} />)
    await user.upload(screen.getByLabelText(es.image.fileInput), file('floral.png', 'image/png'))

    await user.click(screen.getByRole('button', { name: es.image.remove }))

    expect(onChange).toHaveBeenLastCalledWith(null)
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:preview')
  })

  it('releases the preview when the picker goes away', async () => {
    const user = userEvent.setup()
    const { unmount } = render(<ImagePicker onChange={vi.fn()} />)
    await user.upload(screen.getByLabelText(es.image.fileInput), file('floral.png', 'image/png'))

    unmount()

    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:preview')
  })
})
