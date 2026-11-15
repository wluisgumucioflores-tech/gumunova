import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import type { SaleInput } from '../../api/sales'
import { es } from '../../i18n/es'
import { clothStockItem, pvcProduct, roll, tableclothProduct } from '../../test/fixtures'
import { SaleForm } from './SaleForm'

type Submit = (input: SaleInput, idempotencyKey: string) => Promise<void>
type User = ReturnType<typeof userEvent.setup>

const t = es.sales.form

const products = [
  pvcProduct({ pvc: { ...pvcProduct().pvc!, reference_price_per_meter: '15.50', reference_price_per_roll: '280.00' } }),
  tableclothProduct({ tablecloth: { ...tableclothProduct().tablecloth!, unit_price: '60.00' } }),
]
const cutRoll = roll({ id: 'roll-2', code: 'ROL-000002', available_length_m: '9.90', is_intact: false })

function renderForm(onSubmit: Submit = vi.fn<Submit>().mockResolvedValue()) {
  render(
    <MemoryRouter>
      <SaleForm
        rolls={[roll(), cutRoll]}
        tablecloths={[clothStockItem({ quantity: '10.00' })]}
        products={products}
        onSubmit={onSubmit}
        onCancel={() => {}}
      />
    </MemoryRouter>,
  )
  return onSubmit
}

const register = () => screen.getByRole('button', { name: t.submit })

describe('SaleForm basics', () => {
  it('warns that a registered sale cannot be edited and starts with no lines', () => {
    renderForm()

    expect(screen.getByText(t.immutableNotice)).toBeInTheDocument()
    expect(screen.getByText(t.noItems)).toBeInTheDocument()
    expect((screen.getByLabelText(t.fields.date) as HTMLInputElement).value).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })

  it('shows an estimated total of 0 with the reminder that the server decides', () => {
    renderForm()

    expect(screen.getByText('Bs 0,00')).toBeInTheDocument()
    expect(screen.getByText(t.summary.hint)).toBeInTheDocument()
  })

  it('disables the kinds of sale that have nothing to sell', () => {
    render(
      <MemoryRouter>
        <SaleForm rolls={[cutRoll]} tablecloths={[]} products={products} onSubmit={vi.fn()} onCancel={() => {}} />
      </MemoryRouter>,
    )

    expect(screen.getByRole('button', { name: t.addMeters })).toBeEnabled()
    expect(screen.getByRole('button', { name: t.addFullRoll })).toBeDisabled() // the only roll was cut
    expect(screen.getByRole('button', { name: t.addTablecloth })).toBeDisabled()
  })

  it('says why a kind of sale is unavailable and where to get stock', () => {
    render(
      <MemoryRouter>
        <SaleForm rolls={[cutRoll]} tablecloths={[]} products={products} onSubmit={vi.fn()} onCancel={() => {}} />
      </MemoryRouter>,
    )

    expect(screen.getByText(t.noIntactRolls)).toBeInTheDocument()
    expect(screen.getByText(t.noTablecloths)).toBeInTheDocument()
    expect(screen.queryByText(t.noRolls)).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: es.purchases.newPurchase })).toHaveAttribute('href', '/compras/nueva')
  })

  it('shows no explanation when everything can be sold', () => {
    renderForm()

    expect(screen.queryByText(t.noTablecloths)).not.toBeInTheDocument()
    expect(screen.queryByText(t.noRolls)).not.toBeInTheDocument()
  })
})

describe('selling meters of a roll', () => {
  async function addMeters(user: User) {
    await user.click(screen.getByRole('button', { name: t.addMeters }))
  }

  it('lists the rolls with their available meters', async () => {
    const user = userEvent.setup()
    renderForm()
    await addMeters(user)

    expect(screen.getByRole('option', { name: t.rollOption('ROL-000001', 'PVC-FLORAL-20', '19,80') })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: t.rollOption('ROL-000002', 'PVC-FLORAL-20', '9,90') })).toBeInTheDocument()
  })

  it('suggests the catalog price, shows what is available and keeps the price editable', async () => {
    const user = userEvent.setup()
    renderForm()
    await addMeters(user)

    await user.selectOptions(screen.getByLabelText(t.rollLabel), 'roll-1')

    expect(screen.getByLabelText(t.pricePerMeter)).toHaveValue('15,50')
    expect(screen.getByText(t.referencePrice('15,50'))).toBeInTheDocument()
    expect(screen.getByText(t.metersHint('19,80'))).toBeInTheDocument()

    await user.clear(screen.getByLabelText(t.pricePerMeter))
    await user.type(screen.getByLabelText(t.pricePerMeter), '18')
    expect(screen.getByLabelText(t.pricePerMeter)).toHaveValue('18')
  })

  it('does not overwrite a price the seller already typed when the roll changes', async () => {
    const user = userEvent.setup()
    renderForm()
    await addMeters(user)
    await user.type(screen.getByLabelText(t.pricePerMeter), '17')

    await user.selectOptions(screen.getByLabelText(t.rollLabel), 'roll-1')

    expect(screen.getByLabelText(t.pricePerMeter)).toHaveValue('17')
  })

  it('shows the line total and the estimated sale total as the numbers are typed', async () => {
    const user = userEvent.setup()
    renderForm()
    await addMeters(user)
    await user.selectOptions(screen.getByLabelText(t.rollLabel), 'roll-1')
    await user.type(screen.getByLabelText(t.meters), '1,25')

    expect(screen.getByText(t.lineTotal('19,38'))).toBeInTheDocument() // 15,50 x 1,25 = 19,375
    expect(screen.getAllByText('Bs 19,38').length).toBeGreaterThan(0)
  })

  it('submits canonical decimals with an idempotency key', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await addMeters(user)
    await user.selectOptions(screen.getByLabelText(t.rollLabel), 'roll-1')
    await user.type(screen.getByLabelText(t.meters), '2,5')
    await user.type(screen.getByLabelText(t.fields.notes, { exact: false }), ' cliente de paso ')

    await user.click(register())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const [input, key] = vi.mocked(onSubmit).mock.calls[0]!
    expect(input.lines).toEqual([{ kind: 'roll_meters', roll_id: 'roll-1', meters: '2.50', unit_price: '15.50' }])
    expect(input.notes).toBe('cliente de paso')
    expect(input.sale_date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(key).toMatch(/^[A-Za-z0-9._:-]{8,100}$/)
  })
})

describe('selling a whole roll', () => {
  it('offers only intact rolls and suggests the price of the whole roll', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addFullRoll }))

    expect(screen.getByRole('option', { name: t.rollOption('ROL-000001', 'PVC-FLORAL-20', '19,80') })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /ROL-000002/ })).not.toBeInTheDocument()
    expect(screen.getByText(t.intactHint)).toBeInTheDocument()

    await user.selectOptions(screen.getByLabelText(t.rollLabel), 'roll-1')

    expect(screen.getByLabelText(t.pricePerRoll)).toHaveValue('280,00')
    expect(screen.getByText(t.lineTotal('280,00'))).toBeInTheDocument()
  })

  it('submits no meters, only the roll and its price', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await user.click(screen.getByRole('button', { name: t.addFullRoll }))
    await user.selectOptions(screen.getByLabelText(t.rollLabel), 'roll-1')
    await user.clear(screen.getByLabelText(t.pricePerRoll))
    await user.type(screen.getByLabelText(t.pricePerRoll), '350')

    await user.click(register())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(vi.mocked(onSubmit).mock.calls[0]![0].lines).toEqual([{ kind: 'full_roll', roll_id: 'roll-1', unit_price: '350.00' }])
  })
})

describe('selling tablecloths', () => {
  it('shows the units available and suggests the unit price', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addTablecloth }))

    expect(screen.getByRole('option', { name: t.productOption('MAN-ROJO-01', 'Mantel rojo', '10') })).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText(t.productLabel), 'cloth-1')

    expect(screen.getByText(t.quantityHint('10'))).toBeInTheDocument()
    expect(screen.getByLabelText(t.pricePerUnit)).toHaveValue('60,00')
  })

  it('submits the units as a whole number', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await user.click(screen.getByRole('button', { name: t.addTablecloth }))
    await user.selectOptions(screen.getByLabelText(t.productLabel), 'cloth-1')
    await user.type(screen.getByLabelText(t.quantity), '4')

    await user.click(register())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(vi.mocked(onSubmit).mock.calls[0]![0].lines).toEqual([
      { kind: 'tablecloth_units', product_id: 'cloth-1', quantity: 4, unit_price: '60.00' },
    ])
  })
})

describe('validation and server answers', () => {
  it('does not submit an incomplete sale and focuses the first problem', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await user.click(screen.getByRole('button', { name: t.addMeters }))

    await user.click(register())

    expect(onSubmit).not.toHaveBeenCalled()
    expect(screen.getByRole('alert')).toHaveTextContent(t.fixErrors)
    expect(screen.getByLabelText(t.rollLabel)).toHaveFocus()
    expect(screen.getByLabelText(t.rollLabel)).toHaveAttribute('aria-invalid', 'true')
  })

  it('asks for at least one product', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()

    await user.click(register())

    expect(onSubmit).not.toHaveBeenCalled()
    expect(screen.getByText(es.fieldErrors.required)).toBeInTheDocument()
  })

  it('shows the server answer about stock on the exact field', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<Submit>().mockRejectedValue(
      new ApiError(422, 'validation_failed', [{ field: 'lines[0].meters', code: 'insufficient_stock' }]),
    )
    renderForm(onSubmit)
    await user.click(screen.getByRole('button', { name: t.addMeters }))
    await user.selectOptions(screen.getByLabelText(t.rollLabel), 'roll-1')
    await user.type(screen.getByLabelText(t.meters), '50')

    await user.click(register())

    expect(await screen.findByText(es.fieldErrors.insufficient_stock)).toBeInTheDocument()
    expect(screen.getByLabelText(t.meters)).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText(t.meters)).toHaveFocus()
  })

  it('explains that a cut roll cannot be sold whole', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<Submit>().mockRejectedValue(
      new ApiError(422, 'validation_failed', [{ field: 'lines[0].roll_id', code: 'roll_not_intact' }]),
    )
    renderForm(onSubmit)
    await user.click(screen.getByRole('button', { name: t.addFullRoll }))
    await user.selectOptions(screen.getByLabelText(t.rollLabel), 'roll-1')
    await user.clear(screen.getByLabelText(t.pricePerRoll))
    await user.type(screen.getByLabelText(t.pricePerRoll), '350')

    await user.click(register())

    expect(await screen.findByText(es.fieldErrors.roll_not_intact)).toBeInTheDocument()
  })

  it('blocks a second submit while the first is in flight', async () => {
    const user = userEvent.setup()
    let finish: () => void = () => {}
    const onSubmit = vi.fn<Submit>(() => new Promise<void>((resolve) => (finish = resolve)))
    renderForm(onSubmit)
    await user.click(screen.getByRole('button', { name: t.addTablecloth }))
    await user.selectOptions(screen.getByLabelText(t.productLabel), 'cloth-1')
    await user.type(screen.getByLabelText(t.quantity), '1')

    await user.click(register())
    const busy = await screen.findByRole('button', { name: t.submitting })
    expect(busy).toBeDisabled()
    await user.click(busy)

    expect(onSubmit).toHaveBeenCalledTimes(1)
    finish()
    await waitFor(() => expect(register()).toBeEnabled())
  })

  it('reuses the same key when the seller retries after a failure', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<Submit>().mockRejectedValueOnce(new ApiError(0, 'network')).mockResolvedValue()
    renderForm(onSubmit)
    await user.click(screen.getByRole('button', { name: t.addTablecloth }))
    await user.selectOptions(screen.getByLabelText(t.productLabel), 'cloth-1')
    await user.type(screen.getByLabelText(t.quantity), '1')

    await user.click(register())
    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
    await user.click(register())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(2))
    expect(vi.mocked(onSubmit).mock.calls[1]![1]).toBe(vi.mocked(onSubmit).mock.calls[0]![1])
  })

  it('removes a line with an icon-only button that has a name', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addTablecloth }))

    const remove = screen.getByRole('button', { name: t.removeLine(1) })
    expect(remove).toHaveTextContent('')
    await user.click(remove)

    expect(screen.getByText(t.noItems)).toBeInTheDocument()
  })
})
