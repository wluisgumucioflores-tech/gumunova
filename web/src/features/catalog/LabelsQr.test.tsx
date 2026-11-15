import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as fetchAll from '../../api/fetchAll'
import { es } from '../../i18n/es'
import { pvcProduct } from '../../test/fixtures'
import { LabelsPage } from './LabelsPage'

vi.mock('../../api/fetchAll')
// Shows what each code carries, as text, so it can be checked.
vi.mock('../../components/QrCode', () => ({ QrCode: ({ value }: { value: string }) => <p data-testid="qr-value">{value}</p> }))
vi.mock('../../components/Barcode', () => ({ Barcode: ({ value }: { value: string }) => <p data-testid="barcode-value">{value}</p> }))

function renderLabels() {
  return render(
    <MemoryRouter initialEntries={['/catalogo/etiquetas?todos=1']}>
      <Routes>
        <Route path="/catalogo/etiquetas" element={<LabelsPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(fetchAll.listAllProducts).mockResolvedValue([pvcProduct()])
})

describe('what a label carries', () => {
  it('puts the product address in the QR, so a phone camera opens it', async () => {
    renderLabels()

    const value = (await screen.findByTestId('qr-value')).textContent
    expect(value).toBe(`${window.location.origin}/producto/P-AAAAAAAAAAAA`)
  })

  it('keeps only the bare code in the barcode, for hardware scanners', async () => {
    const user = userEvent.setup()
    renderLabels()

    await user.selectOptions(await screen.findByLabelText(es.labels.kindLabel), 'barcode')

    expect(screen.getByTestId('barcode-value')).toHaveTextContent(/^P-AAAAAAAAAAAA$/)
  })

  it('warns that a QR pointing to this computer cannot be opened from a phone', async () => {
    renderLabels() // the tests run at localhost

    expect(await screen.findByText(es.labels.qrLocalWarning(window.location.origin))).toBeInTheDocument()
  })
})
