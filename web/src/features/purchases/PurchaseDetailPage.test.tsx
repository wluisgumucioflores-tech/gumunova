import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as api from '../../api/purchases'
import { es } from '../../i18n/es'
import { purchase } from '../../test/fixtures'
import { PurchaseDetailPage } from './PurchaseDetailPage'

vi.mock('../../api/purchases')

const getPurchase = vi.mocked(api.getPurchase)
const t = es.purchases.detail

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/compras/pur-1']}>
      <Routes>
        <Route path="/compras/:id" element={<PurchaseDetailPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('PurchaseDetailPage', () => {
  it('shows the purchase header with Spanish dates and amounts', async () => {
    getPurchase.mockResolvedValue(purchase())

    renderPage()

    const header = await screen.findByRole('region', { name: t.title })
    expect(header).toHaveTextContent('Plásticos del Sur')
    expect(header).toHaveTextContent('10/11/2026')
    expect(header).toHaveTextContent('FAC-001')
    expect(header).toHaveTextContent(t.status.confirmed)
    expect(header).toHaveTextContent('Bs 30,00')
    expect(header).toHaveTextContent('Bs 910,00')
    expect(header).toHaveTextContent('Entrega parcial')
    expect(header).toHaveTextContent(t.editableNote)
  })

  it('offers editing, and explains when the items can no longer change', async () => {
    getPurchase.mockResolvedValue(purchase({ items_editable: false, revision: 3 }))

    renderPage()

    const header = await screen.findByRole('region', { name: t.title })
    expect(header).toHaveTextContent(t.lockedNote)
    expect(header).toHaveTextContent(t.revisionNote(3))
    expect(screen.getByRole('link', { name: es.purchases.edit })).toHaveAttribute('href', '/compras/pur-1/editar')
  })

  it('says so when the purchase has no supplier', async () => {
    getPurchase.mockResolvedValue(purchase({ supplier: null }))

    renderPage()

    const header = await screen.findByRole('region', { name: t.title })
    expect(header).toHaveTextContent(es.purchases.noSupplier)
  })

  it('lists every physical roll created, with its code and attributable cost', async () => {
    getPurchase.mockResolvedValue(purchase())

    renderPage()

    const pvc = await screen.findByRole('region', { name: t.lineHeading(1, 'PVC floral 20 m') })
    const rows = within(pvc).getAllByRole('row')
    expect(rows).toHaveLength(3) // header + two rolls
    expect(within(rows[1]!).getByText('ROL-000001')).toBeInTheDocument()
    expect(rows[1]).toHaveTextContent('19,80 m')
    expect(rows[1]).toHaveTextContent('Bs 280,00')
    expect(rows[1]).toHaveTextContent('Bs 9,54')
    expect(rows[1]).toHaveTextContent('Bs 289,54')
    expect(within(rows[2]!).getByText('ROL-000002')).toBeInTheDocument()
    expect(rows[2]).toHaveTextContent('Bs 310,23')
  })

  it('shows tablecloth lines as units added to stock, without rolls', async () => {
    getPurchase.mockResolvedValue(purchase())

    renderPage()

    const cloth = await screen.findByRole('region', { name: t.lineHeading(2, 'Mantel rojo') })
    expect(cloth).toHaveTextContent('MAN-ROJO-01')
    expect(cloth).toHaveTextContent('10')
    expect(cloth).toHaveTextContent('Bs 30,00')
    expect(cloth).toHaveTextContent('Bs 310,23')
    expect(cloth).toHaveTextContent(t.noRollsNote)
    expect(within(cloth).queryByRole('table')).not.toBeInTheDocument()
  })

  it('has no delete action: a registered purchase is corrected, never removed', async () => {
    getPurchase.mockResolvedValue(purchase())

    renderPage()
    await screen.findByRole('region', { name: t.title })

    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('shows a loading state first', () => {
    getPurchase.mockReturnValue(new Promise(() => {}))

    renderPage()

    expect(screen.getByRole('status')).toHaveTextContent(es.common.loading)
  })

  it('shows a Spanish error with a retry when the purchase does not exist', async () => {
    const user = userEvent.setup()
    getPurchase.mockRejectedValueOnce(new ApiError(404, 'purchase_not_found')).mockResolvedValueOnce(purchase())

    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.purchase_not_found)
    await user.click(screen.getByRole('button', { name: es.common.retry }))
    expect(await screen.findByRole('region', { name: t.title })).toBeInTheDocument()
  })
})
