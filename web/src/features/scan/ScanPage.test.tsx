import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as api from '../../api/labels'
import * as scanner from '../../lib/scanner'
import { es } from '../../i18n/es'
import { pvcProduct, roll, tableclothProduct } from '../../test/fixtures'
import { ScanPage } from './ScanPage'

vi.mock('../../api/labels')
vi.mock('../../lib/scanner')
// The real component needs a camera; this one reads "a code" when its button is pressed.
vi.mock('../../components/CameraScanner', () => ({
  CameraScanner: ({ onCode, onError }: { onCode: (text: string) => void; onError: () => void }) => (
    <div>
      <button type="button" onClick={() => onCode('P-AAAAAAAAAAAA')}>
        leer-código
      </button>
      <button type="button" onClick={onError}>
        fallar-cámara
      </button>
    </div>
  ),
}))

const t = es.scan
const lookupLabel = vi.mocked(api.lookupLabel)

function renderPage(address = '/escanear') {
  return render(
    <MemoryRouter initialEntries={[address]}>
      <Routes>
        <Route path="/escanear" element={<ScanPage />} />
        <Route path="/producto/:code" element={<ScanPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(scanner.cameraAvailable).mockReturnValue(true)
})

describe('ScanPage', () => {
  it('has the code field ready for a hardware scanner and looks the code up on Enter', async () => {
    const user = userEvent.setup()
    lookupLabel.mockResolvedValue({ kind: 'product', product: pvcProduct() })
    renderPage()

    expect(screen.getByLabelText(t.manualLabel)).toHaveFocus()
    await user.type(screen.getByLabelText(t.manualLabel), 'P-AAAAAAAAAAAA{Enter}')

    expect(lookupLabel).toHaveBeenCalledWith('P-AAAAAAAAAAAA', expect.anything())
    expect(await screen.findByRole('heading', { name: 'PVC floral 20 m' })).toBeInTheDocument()
    expect(screen.getByLabelText(t.manualLabel)).toHaveValue('')
  })

  it('shows the name and both prices of a PVC product', async () => {
    const user = userEvent.setup()
    lookupLabel.mockResolvedValue({ kind: 'product', product: pvcProduct() })
    renderPage()

    await user.type(screen.getByLabelText(t.manualLabel), 'P-AAAAAAAAAAAA{Enter}')

    const result = await screen.findByRole('region', { name: t.resultLabel })
    expect(result).toHaveTextContent('PVC floral 20 m')
    expect(result).toHaveTextContent(es.catalog.types.pvc)
    expect(result).toHaveTextContent('PVC-FLORAL-20')
    expect(within(result).getByText(t.perMeter).nextElementSibling).toHaveTextContent('Bs 15,50')
    expect(within(result).getByText(t.perRoll).nextElementSibling).toHaveTextContent('Bs 1.280,00')
    expect(within(result).queryByText(t.perUnit)).not.toBeInTheDocument()
  })

  it('shows the unit price of a tablecloth', async () => {
    const user = userEvent.setup()
    lookupLabel.mockResolvedValue({ kind: 'product', product: tableclothProduct() })
    renderPage()

    await user.type(screen.getByLabelText(t.manualLabel), 'P-BBBBBBBBBBBB{Enter}')

    const result = await screen.findByRole('region', { name: t.resultLabel })
    expect(result).toHaveTextContent('Mantel rojo')
    expect(result).toHaveTextContent(es.catalog.types.tablecloth)
    expect(within(result).getByText(t.perUnit).nextElementSibling).toHaveTextContent('Bs 60,00')
    expect(within(result).queryByText(t.perMeter)).not.toBeInTheDocument()
  })

  it('for a roll, adds the roll, its meters and whether it can be sold whole', async () => {
    const user = userEvent.setup()
    lookupLabel.mockResolvedValue({ kind: 'roll', product: pvcProduct(), roll: roll({ available_length_m: '12.50', is_intact: false }) })
    renderPage()

    await user.type(screen.getByLabelText(t.manualLabel), 'R-111111111111{Enter}')

    const result = await screen.findByRole('region', { name: t.resultLabel })
    expect(result).toHaveTextContent(t.rollHeading('ROL-000001'))
    expect(result).toHaveTextContent('12,50 m')
    expect(result).toHaveTextContent(t.rollCutHint)
    expect(result).toHaveTextContent('Bs 15,50')
  })

  it('flags a deactivated product', async () => {
    const user = userEvent.setup()
    lookupLabel.mockResolvedValue({ kind: 'product', product: pvcProduct({ is_active: false }) })
    renderPage()

    await user.type(screen.getByLabelText(t.manualLabel), 'P-AAAAAAAAAAAA{Enter}')

    expect(await screen.findByText(es.catalog.status.inactive)).toBeInTheDocument()
  })

  it('says when the code is unknown or outdated, and lets the person scan again', async () => {
    const user = userEvent.setup()
    lookupLabel.mockRejectedValue(new ApiError(404, 'label_not_found'))
    renderPage()

    await user.type(screen.getByLabelText(t.manualLabel), 'P-FFFFFFFFFFFF{Enter}')

    expect(await screen.findByRole('heading', { name: t.unknownTitle })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: t.scanAnother }))
    expect(screen.queryByRole('heading', { name: t.unknownTitle })).not.toBeInTheDocument()
    expect(screen.getByLabelText(t.manualLabel)).toHaveFocus()
  })

  it('shows a Spanish error when the lookup fails for another reason', async () => {
    const user = userEvent.setup()
    lookupLabel.mockRejectedValue(new ApiError(0, 'network'))
    renderPage()

    await user.type(screen.getByLabelText(t.manualLabel), 'P-AAAAAAAAAAAA{Enter}')

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
  })

  it('does not search for an empty code', () => {
    renderPage()

    expect(screen.getByRole('button', { name: t.search })).toBeDisabled()
  })
})

describe('ScanPage camera and photo', () => {
  it('reads from the camera, closes it and shows the result', async () => {
    const user = userEvent.setup()
    lookupLabel.mockResolvedValue({ kind: 'product', product: pvcProduct() })
    renderPage()

    await user.click(screen.getByRole('button', { name: t.useCamera }))
    await user.click(screen.getByRole('button', { name: 'leer-código' }))

    expect(lookupLabel).toHaveBeenCalledWith('P-AAAAAAAAAAAA', expect.anything())
    expect(await screen.findByRole('region', { name: t.resultLabel })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'leer-código' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: t.useCamera })).toBeInTheDocument()
  })

  it('explains when the camera cannot be opened', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(screen.getByRole('button', { name: t.useCamera }))
    await user.click(screen.getByRole('button', { name: 'fallar-cámara' }))

    expect(screen.getByRole('alert')).toHaveTextContent(t.cameraFailed)
    expect(screen.getByLabelText(t.manualLabel)).toBeInTheDocument()
  })

  it('offers only the photo and the keyboard when the camera is not allowed here', () => {
    vi.mocked(scanner.cameraAvailable).mockReturnValue(false)
    renderPage()

    expect(screen.queryByRole('button', { name: t.useCamera })).not.toBeInTheDocument()
    expect(screen.getByText(t.cameraUnavailable)).toBeInTheDocument()
    expect(screen.getByText(t.takePhoto)).toBeInTheDocument()
  })

  it('reads the code from a photo', async () => {
    const user = userEvent.setup()
    vi.mocked(scanner.readCodeFromImageFile).mockResolvedValue('P-AAAAAAAAAAAA')
    lookupLabel.mockResolvedValue({ kind: 'product', product: pvcProduct() })
    renderPage()

    await user.upload(screen.getByLabelText(t.takePhoto), new File(['x'], 'etiqueta.jpg', { type: 'image/jpeg' }))

    expect(await screen.findByRole('region', { name: t.resultLabel })).toBeInTheDocument()
    expect(lookupLabel).toHaveBeenCalledWith('P-AAAAAAAAAAAA', expect.anything())
  })

  it('says so when the photo has no readable code', async () => {
    const user = userEvent.setup()
    vi.mocked(scanner.readCodeFromImageFile).mockResolvedValue(null)
    renderPage()

    await user.upload(screen.getByLabelText(t.takePhoto), new File(['x'], 'borrosa.jpg', { type: 'image/jpeg' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(t.noCodeInPhoto)
    expect(lookupLabel).not.toHaveBeenCalled()
  })
})

describe('ScanPage opened from a label', () => {
  it('looks the code of the address up right away and shows the product', async () => {
    lookupLabel.mockResolvedValue({ kind: 'product', product: pvcProduct() })

    renderPage('/producto/P-AAAAAAAAAAAA')

    expect(await screen.findByRole('region', { name: t.resultLabel })).toHaveTextContent('PVC floral 20 m')
    expect(lookupLabel).toHaveBeenCalledTimes(1)
    expect(lookupLabel).toHaveBeenCalledWith('P-AAAAAAAAAAAA', expect.anything())
  })

  it('says the code is unknown when the address carries an old one', async () => {
    lookupLabel.mockRejectedValue(new ApiError(404, 'label_not_found'))

    renderPage('/producto/P-FFFFFFFFFFFF')

    expect(await screen.findByRole('heading', { name: t.unknownTitle })).toBeInTheDocument()
  })

  it('understands a pasted or scanned address, not only a bare code', async () => {
    const user = userEvent.setup()
    lookupLabel.mockResolvedValue({ kind: 'product', product: pvcProduct() })
    renderPage()

    await user.type(screen.getByLabelText(t.manualLabel), 'https://gumunova.example.com/producto/P-AAAAAAAAAAAA{Enter}')

    expect(lookupLabel).toHaveBeenCalledWith('P-AAAAAAAAAAAA', expect.anything())
  })
})

