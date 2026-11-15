import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import * as api from '../../api/inventory'
import { es } from '../../i18n/es'
import { pageOf, roll } from '../../test/fixtures'
import { RollsPage } from './RollsPage'

vi.mock('../../api/inventory')

const listRolls = vi.mocked(api.listRolls)
const r = es.inventory.rolls

function renderPage(path = '/inventario/rollos') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <RollsPage />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

describe('RollsPage', () => {
  it('shows each roll with its own length, cost and cost per meter', async () => {
    listRolls.mockResolvedValue(pageOf([roll()]))

    renderPage()

    const row = (await screen.findByText('ROL-000001')).closest('tr')!
    expect(within(row).getByText('PVC-FLORAL-20 — PVC floral 20 m')).toBeInTheDocument()
    expect(within(row).getByText('10/11/2026')).toBeInTheDocument()
    expect(within(row).getAllByText('19,80 m')).toHaveLength(2)
    expect(within(row).getAllByText('Bs 289,54')).toHaveLength(2)
    expect(within(row).getByText('Bs 14,6232')).toBeInTheDocument()
    expect(within(row).getByText(r.intact)).toBeInTheDocument()
  })

  it('tells intact, cut and depleted rolls apart', async () => {
    listRolls.mockResolvedValue(
      pageOf([
        roll(),
        roll({ id: 'r2', code: 'ROL-000002', available_length_m: '9.90', is_intact: false }),
        roll({ id: 'r3', code: 'ROL-000003', available_length_m: '0.00', is_intact: false }),
      ]),
    )

    renderPage()

    expect(within((await screen.findByText('ROL-000002')).closest('tr')!).getByText(r.cut)).toBeInTheDocument()
    expect(within(screen.getByText('ROL-000003').closest('tr')!).getByText(r.depleted)).toBeInTheDocument()
  })

  it('links to the roll movements and to its purchase', async () => {
    listRolls.mockResolvedValue(pageOf([roll()]))

    renderPage()

    expect(await screen.findByRole('link', { name: `${r.viewMovements} ROL-000001` })).toHaveAttribute(
      'href',
      '/inventario/movimientos?rollo=roll-1',
    )
    expect(screen.getByRole('link', { name: `${r.viewPurchase} ROL-000001` })).toHaveAttribute('href', '/compras/pur-1')
  })

  it('lists only available rolls by default and sends the chosen status', async () => {
    listRolls.mockResolvedValue(pageOf([roll()]))
    const user = userEvent.setup()
    renderPage()
    await screen.findByText('ROL-000001')
    expect(listRolls).toHaveBeenLastCalledWith(
      { productId: undefined, status: 'available', query: '', limit: 25, offset: 0 },
      expect.any(AbortSignal),
    )

    await user.selectOptions(screen.getByLabelText(r.statusLabel), 'all')

    await vi.waitFor(() =>
      expect(listRolls).toHaveBeenLastCalledWith(
        { productId: undefined, status: 'all', query: '', limit: 25, offset: 0 },
        expect.any(AbortSignal),
      ),
    )
  })

  it('filters by the product in the address and lets the user remove that filter', async () => {
    listRolls.mockResolvedValue(pageOf([roll()]))
    const user = userEvent.setup()
    renderPage('/inventario/rollos?producto=pvc-1')

    expect(await screen.findByText(new RegExp(r.filteredByProduct))).toBeInTheDocument()
    expect(listRolls).toHaveBeenLastCalledWith(expect.objectContaining({ productId: 'pvc-1' }), expect.any(AbortSignal))

    await user.click(screen.getByRole('button', { name: r.clearProduct }))

    await vi.waitFor(() => expect(listRolls).toHaveBeenLastCalledWith(expect.objectContaining({ productId: undefined }), expect.any(AbortSignal)))
    expect(screen.queryByText(new RegExp(r.filteredByProduct))).not.toBeInTheDocument()
  })

  it('explains when there are no rolls', async () => {
    listRolls.mockResolvedValue(pageOf([]))

    renderPage()

    expect(await screen.findByText(r.emptyTitle)).toBeInTheDocument()
  })
})

describe('RollsPage labels', () => {
  it('links to the labels of the ticked rolls and of all the available ones', async () => {
    const user = userEvent.setup()
    listRolls.mockResolvedValue(pageOf([roll(), roll({ id: 'roll-2', code: 'ROL-000002' })]))
    renderPage()

    expect(await screen.findByRole('link', { name: r.labels.openAll })).toHaveAttribute('href', '/inventario/rollos/etiquetas?todos=1')
    await user.click(screen.getByRole('checkbox', { name: r.labels.selectRow('ROL-000002') }))

    const bar = screen.getByRole('group', { name: r.labels.selectedCount(1) })
    expect(within(bar).getByRole('link', { name: r.labels.open })).toHaveAttribute('href', '/inventario/rollos/etiquetas?ids=roll-2')
    await user.click(screen.getByRole('checkbox', { name: r.labels.selectPage }))
    expect(screen.getByRole('link', { name: r.labels.open })).toHaveAttribute('href', '/inventario/rollos/etiquetas?ids=roll-2,roll-1')
  })

  it('regenerates a roll code only after confirming, and says to print the label again', async () => {
    const user = userEvent.setup()
    listRolls.mockResolvedValue(pageOf([roll()]))
    vi.mocked(api.resetRollLabelCode).mockResolvedValue(roll({ label_code: 'R-NEW' }))
    renderPage()

    await user.click(await screen.findByRole('button', { name: `${r.labels.reset} ROL-000001` }))
    expect(api.resetRollLabelCode).not.toHaveBeenCalled()
    expect(screen.getByText(r.labels.confirmReset('ROL-000001'))).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: r.labels.confirmResetYes }))

    expect(api.resetRollLabelCode).toHaveBeenCalledWith('roll-1')
    expect(await screen.findByText(r.labels.resetDone('ROL-000001'))).toBeInTheDocument()
  })

  it('does nothing when the regeneration is cancelled, and explains a failure', async () => {
    const user = userEvent.setup()
    listRolls.mockResolvedValue(pageOf([roll()]))
    vi.mocked(api.resetRollLabelCode).mockRejectedValue(new ApiError(404, 'roll_not_found'))
    renderPage()

    await user.click(await screen.findByRole('button', { name: `${r.labels.reset} ROL-000001` }))
    await user.click(screen.getByRole('button', { name: es.common.cancel }))
    expect(api.resetRollLabelCode).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: `${r.labels.reset} ROL-000001` }))
    await user.click(screen.getByRole('button', { name: r.labels.confirmResetYes }))
    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.roll_not_found)
  })
})

