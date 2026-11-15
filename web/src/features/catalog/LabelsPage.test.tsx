import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as fetchAll from '../../api/fetchAll'
import { es } from '../../i18n/es'
import { pvcProduct, tableclothProduct } from '../../test/fixtures'
import { LabelsPage } from './LabelsPage'

vi.mock('../../api/fetchAll')

const t = es.catalog.labels
const c = es.labels
const print = vi.fn()

function renderAt(search: string) {
  return render(
    <MemoryRouter initialEntries={[`/catalogo/etiquetas${search}`]}>
      <Routes>
        <Route path="/catalogo/etiquetas" element={<LabelsPage />} />
        <Route path="/catalogo" element={<p>catálogo</p>} />
      </Routes>
    </MemoryRouter>,
  )
}

const pvc = pvcProduct()
const cloth = tableclothProduct()

beforeEach(() => {
  vi.resetAllMocks()
  print.mockReset()
  vi.stubGlobal('print', print)
  vi.mocked(fetchAll.listAllProducts).mockResolvedValue([pvc, cloth])
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('LabelsPage', () => {
  it('prints a QR label for each selected product, with its SKU and name', async () => {
    renderAt(`?ids=${cloth.id}`)

    const sheet = await screen.findByRole('list', { name: t.title })
    const labels = within(sheet).getAllByRole('listitem')
    expect(labels).toHaveLength(1)
    expect(labels[0]).toHaveTextContent('MAN-ROJO-01')
    expect(labels[0]).toHaveTextContent('Mantel rojo')
    // The code the image carries is the product's own label code, shown in small print, not its SKU.
    expect(labels[0]).toHaveTextContent('P-BBBBBBBBBBBB')
    expect(within(labels[0]!).getByRole('img', { name: c.codeAlt('MAN-ROJO-01') })).toBeInTheDocument()
    expect(labels[0]!.querySelector('path')?.getAttribute('d')).toMatch(/^M\d+ \d+h1v1h-1z/) // the QR modules
    expect(fetchAll.listAllProducts).toHaveBeenCalledWith(undefined, expect.anything())
  })

  it('switches to Code 128 barcodes', async () => {
    const user = userEvent.setup()
    renderAt(`?ids=${pvc.id}`)

    await user.selectOptions(await screen.findByLabelText(c.kindLabel), 'barcode')

    const image = screen.getByRole('img', { name: c.codeAlt('PVC-FLORAL-20') })
    expect(image.querySelectorAll('rect').length).toBeGreaterThan(10) // one bar per rect
    expect(image).toHaveAttribute('viewBox')
  })

  it('repeats each label by the number of copies', async () => {
    const user = userEvent.setup()
    renderAt(`?ids=${pvc.id},${cloth.id}`)

    const copies = await screen.findByLabelText(c.copiesLabel)
    await user.clear(copies)
    await user.type(copies, '3')

    expect(within(screen.getByRole('list', { name: t.title })).getAllByRole('listitem')).toHaveLength(6)
    expect(screen.getByText(t.summary(2, 6))).toBeInTheDocument()
  })

  it('keeps the copies within 1 and the maximum', async () => {
    const user = userEvent.setup()
    renderAt(`?ids=${pvc.id}`)

    const copies = await screen.findByLabelText(c.copiesLabel)
    await user.clear(copies)
    await user.type(copies, '99')
    await user.tab()

    expect(copies).toHaveValue('50')
  })

  it('prints every active product with "todos" and opens the print dialog', async () => {
    const user = userEvent.setup()
    renderAt('?todos=1')

    await user.click(await screen.findByRole('button', { name: c.print }))

    expect(fetchAll.listAllProducts).toHaveBeenCalledWith(true, expect.anything())
    expect(within(screen.getByRole('list', { name: t.title })).getAllByRole('listitem')).toHaveLength(2)
    expect(print).toHaveBeenCalledTimes(1)
  })

  it('says so when a chosen product no longer exists', async () => {
    renderAt(`?ids=${pvc.id},gone-1`)

    expect(await screen.findByText(t.someMissing(1))).toBeInTheDocument()
    expect(within(screen.getByRole('list', { name: t.title })).getAllByRole('listitem')).toHaveLength(1)
  })

  it('asks to choose products when none were requested', async () => {
    renderAt('')

    expect(await screen.findByText(t.nothingSelected)).toBeInTheDocument()
    expect(fetchAll.listAllProducts).toHaveBeenCalledTimes(0)
  })

  it('shows a Spanish error with a retry when the products cannot be loaded', async () => {
    const user = userEvent.setup()
    vi.mocked(fetchAll.listAllProducts).mockRejectedValueOnce(new ApiError(0, 'network'))
    renderAt('?todos=1')

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
    await user.click(screen.getByRole('button', { name: es.common.retry }))

    expect(await screen.findByRole('list', { name: t.title })).toBeInTheDocument()
  })
})
