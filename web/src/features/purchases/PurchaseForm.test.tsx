import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import type { PurchaseInput } from '../../api/purchases'
import { es } from '../../i18n/es'
import { pvcProduct, supplier, tableclothProduct } from '../../test/fixtures'
import { PurchaseForm } from './PurchaseForm'

const t = es.purchases.form
type User = ReturnType<typeof userEvent.setup>
type Submit = (input: PurchaseInput, key: string) => Promise<void>

function renderForm(onSubmit: Submit = vi.fn<Submit>().mockResolvedValue()) {
  render(
    <PurchaseForm
      suppliers={[supplier()]}
      pvcProducts={[pvcProduct()]}
      tableclothProducts={[tableclothProduct()]}
      onSubmit={onSubmit}
      onCancel={() => {}}
    />,
  )
  return onSubmit
}

const register = () => screen.getByRole('button', { name: t.submit })

async function fillCompletePurchase(user: User) {
  await user.selectOptions(screen.getByLabelText(t.fields.supplier), 'sup-1')
  await user.type(screen.getByLabelText(t.fields.reference, { exact: false }), 'FAC-001')

  await user.click(screen.getByRole('button', { name: t.addPvc }))
  await user.selectOptions(screen.getAllByLabelText(t.productLabel)[0]!, 'pvc-1')
  await user.type(screen.getByLabelText(t.rollLength), '19,8')
  await user.type(screen.getByLabelText(t.rollCost), '280')

  await user.click(screen.getByRole('button', { name: t.addTablecloth }))
  await user.selectOptions(screen.getAllByLabelText(t.productLabel)[1]!, 'cloth-1')
  await user.type(screen.getByLabelText(t.quantity), '10')
  await user.type(screen.getByLabelText(t.unitCost), '30')

  await user.type(screen.getByLabelText(t.fields.additionalCosts, { exact: false }), '30')
}

describe('PurchaseForm basics', () => {
  it('explains what registering does and starts empty', () => {
    renderForm()

    expect(screen.getByText(t.registerNotice)).toBeInTheDocument()
    expect(screen.getByText(t.noItems)).toBeInTheDocument()
    expect((screen.getByLabelText(t.fields.date) as HTMLInputElement).value).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(screen.getByRole('region', { name: t.sections.summary })).toHaveTextContent('Bs 0,00')
  })

  it('offers the suppliers and the products of each kind as options', async () => {
    const user = userEvent.setup()
    renderForm()

    expect(within(screen.getByLabelText(t.fields.supplier)).getByRole('option', { name: 'Plásticos del Sur' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    expect(screen.getByRole('option', { name: 'PVC-FLORAL-20 — PVC floral 20 m' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /MAN-ROJO-01/ })).not.toBeInTheDocument()
  })

  it('requires at least one product and focuses the first problem', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()

    await user.click(register())

    expect(onSubmit).not.toHaveBeenCalled()
    expect(screen.getAllByRole('alert').length).toBeGreaterThan(0)
    expect(screen.getAllByText(es.fieldErrors.required)).toHaveLength(1)
    expect(screen.getByRole('button', { name: t.addPvc })).toBeInTheDocument()
  })

  it('reports an incomplete roll at the exact field', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await user.selectOptions(screen.getByLabelText(t.fields.supplier), 'sup-1')
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    await user.selectOptions(screen.getByLabelText(t.productLabel), 'pvc-1')
    await user.type(screen.getByLabelText(t.rollLength), '0')

    await user.click(register())

    expect(onSubmit).not.toHaveBeenCalled()
    expect(screen.getByText(es.fieldErrors.must_be_positive)).toBeInTheDocument()
    expect(screen.getByLabelText(t.rollLength)).toHaveFocus()
    expect(screen.getByLabelText(t.rollCost)).toHaveAttribute('aria-invalid', 'true')
  })
})

describe('PurchaseForm lines and rolls', () => {
  it('adds a roll copying the previous one, and lets rolls be removed down to one', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    await user.type(screen.getByLabelText(t.rollLength), '20')
    await user.type(screen.getByLabelText(t.rollCost), '100')

    expect(screen.getByRole('button', { name: t.removeRoll(1, 1) })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: t.addRoll }))

    const lengths = screen.getAllByLabelText(t.rollLength)
    expect(lengths).toHaveLength(2)
    expect(lengths[1]).toHaveValue('20')
    expect(screen.getAllByLabelText(t.rollCost)[1]).toHaveValue('100')

    await user.click(screen.getByRole('button', { name: t.removeRoll(1, 2) }))
    expect(screen.getAllByLabelText(t.rollLength)).toHaveLength(1)
  })

  it('removes a whole line', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addTablecloth }))
    expect(screen.getByRole('region', { name: t.tableclothLineTitle(1) })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: t.removeLine(1) }))

    expect(screen.queryByRole('region', { name: t.tableclothLineTitle(1) })).not.toBeInTheDocument()
    expect(screen.getByText(t.noItems)).toBeInTheDocument()
  })

  it('keeps what was typed in other rows when a row is removed', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    await user.type(screen.getByLabelText(t.rollLength), '19')
    await user.click(screen.getByRole('button', { name: t.addRoll }))
    await user.clear(screen.getAllByLabelText(t.rollLength)[1]!)
    await user.type(screen.getAllByLabelText(t.rollLength)[1]!, '25')

    await user.click(screen.getByRole('button', { name: t.removeRoll(1, 1) }))

    expect(screen.getByLabelText(t.rollLength)).toHaveValue('25')
  })

  it('updates the estimated totals exactly while typing', async () => {
    const user = userEvent.setup()
    renderForm()
    await fillCompletePurchase(user)

    // 280 + 10 x 30 = 580 of goods, plus 30 of additional costs.
    const summary = screen.getByRole('region', { name: t.sections.summary })
    expect(summary).toHaveTextContent('Bs 580,00')
    expect(summary).toHaveTextContent('Bs 30,00')
    expect(summary).toHaveTextContent('Bs 610,00')
  })
})

describe('PurchaseForm submission', () => {
  it('submits without a supplier when none is chosen', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await fillCompletePurchase(user)
    await user.selectOptions(screen.getByLabelText(t.fields.supplier), '')

    await user.click(register())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(vi.mocked(onSubmit).mock.calls[0]![0].supplier_id).toBe('')
  })

  it('submits canonical decimals with an idempotency key', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await fillCompletePurchase(user)

    await user.click(register())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const [input, key] = vi.mocked(onSubmit).mock.calls[0]!
    expect(input).toMatchObject({
      supplier_id: 'sup-1',
      document_reference: 'FAC-001',
      additional_costs: '30.00',
      lines: [
        { product_id: 'pvc-1', rolls: [{ length_m: '19.80', cost: '280.00' }] },
        { product_id: 'cloth-1', quantity: 10, unit_cost: '30.00' },
      ],
    })
    expect(input.purchase_date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    expect(key).toMatch(/^[A-Za-z0-9._:-]{8,100}$/)
  })

  it('retries with the same key after a failure, so a timeout can never register twice', async () => {
    const user = userEvent.setup()
    const onSubmit = vi
      .fn<Submit>()
      .mockRejectedValueOnce(new ApiError(0, 'network'))
      .mockResolvedValueOnce(undefined)
    renderForm(onSubmit)
    await fillCompletePurchase(user)

    await user.click(register())
    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
    await user.click(register())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(2))
    expect(onSubmit.mock.calls[1]![1]).toBe(onSubmit.mock.calls[0]![1])
  })

  it('blocks a second submit while the first is in flight', async () => {
    const user = userEvent.setup()
    let finish: () => void = () => {}
    const onSubmit = vi.fn<Submit>(() => new Promise<void>((resolve) => (finish = resolve)))
    renderForm(onSubmit)
    await fillCompletePurchase(user)

    await user.click(register())
    const busy = await screen.findByRole('button', { name: t.submitting })
    expect(busy).toBeDisabled()
    await user.click(busy)

    expect(onSubmit).toHaveBeenCalledTimes(1)
    finish()
    await waitFor(() => expect(register()).toBeEnabled())
  })

  it('shows server-side errors at the exact nested field', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<Submit>().mockRejectedValue(
      new ApiError(422, 'validation_failed', [
        { field: 'lines[0].rolls[0].length_m', code: 'must_be_positive' },
        { field: 'supplier_id', code: 'inactive' },
      ]),
    )
    renderForm(onSubmit)
    await fillCompletePurchase(user)

    await user.click(register())

    expect(await screen.findByText(es.fieldErrors.inactive)).toBeInTheDocument()
    expect(screen.getByText(es.fieldErrors.must_be_positive)).toBeInTheDocument()
    expect(screen.getByLabelText(t.rollLength)).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText(t.fields.supplier)).toHaveFocus()
  })

  it('explains a key that was already used for another purchase', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<Submit>().mockRejectedValue(new ApiError(409, 'idempotency_key_reused'))
    renderForm(onSubmit)
    await fillCompletePurchase(user)

    await user.click(register())

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.idempotency_key_reused)
  })

  it('calls onCancel', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    render(
      <PurchaseForm suppliers={[supplier()]} pvcProducts={[]} tableclothProducts={[]} onSubmit={vi.fn()} onCancel={onCancel} />,
    )

    await user.click(screen.getByRole('button', { name: es.common.cancel }))

    expect(onCancel).toHaveBeenCalled()
  })
})

describe('creating a product from the purchase', () => {
  async function chooseNewProduct(user: User) {
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    await user.selectOptions(screen.getByLabelText(t.productLabel), t.newProductOption)
  }

  it('shows the product fields only after choosing a new product', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    expect(screen.queryByLabelText(es.form.fields.sku)).not.toBeInTheDocument()

    await user.selectOptions(screen.getByLabelText(t.productLabel), t.newProductOption)

    expect(screen.getByLabelText(es.form.fields.sku)).toBeInTheDocument()
    expect(screen.getByLabelText(es.form.fields.widthM)).toBeInTheDocument()
    expect(screen.getByText(t.newProductHint)).toBeInTheDocument()

    await user.selectOptions(screen.getByLabelText(t.productLabel), 'pvc-1')
    expect(screen.queryByLabelText(es.form.fields.sku)).not.toBeInTheDocument()
  })

  it('does not register until the new product is complete, and points at its fields', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await chooseNewProduct(user)
    await user.type(screen.getByLabelText(t.rollLength), '19,8')
    await user.type(screen.getByLabelText(t.rollCost), '280')

    await user.click(register())

    expect(onSubmit).not.toHaveBeenCalled()
    expect(screen.getByLabelText(es.form.fields.sku)).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText(es.form.fields.sku)).toHaveFocus()
  })

  it('submits the new product with the line', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await chooseNewProduct(user)
    await user.type(screen.getByLabelText(es.form.fields.sku), 'pvc-nuevo')
    await user.type(screen.getByLabelText(es.form.fields.name), 'PVC nuevo')
    await user.type(screen.getByLabelText(es.form.fields.widthM), '1,4')
    await user.type(screen.getByLabelText(es.form.fields.nominalLengthM), '20')
    await user.type(screen.getByLabelText(es.form.fields.pricePerMeter), '15')
    await user.type(screen.getByLabelText(es.form.fields.pricePerRoll), '280')
    await user.type(screen.getByLabelText(t.rollLength), '19,8')
    await user.type(screen.getByLabelText(t.rollCost), '280')

    await user.click(register())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const line = vi.mocked(onSubmit).mock.calls[0]![0].lines[0]!
    expect(line).not.toHaveProperty('product_id')
    expect(line.new_product).toMatchObject({ sku: 'pvc-nuevo', name: 'PVC nuevo', product_type: 'pvc' })
  })

  it('shows a taken SKU on the SKU field of the line', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<Submit>().mockRejectedValue(
      new ApiError(422, 'validation_failed', [{ field: 'lines[0].new_product.sku', code: 'already_exists' }]),
    )
    renderForm(onSubmit)
    await chooseNewProduct(user)
    await user.type(screen.getByLabelText(es.form.fields.sku), 'pvc-nuevo')
    await user.type(screen.getByLabelText(es.form.fields.name), 'PVC nuevo')
    await user.type(screen.getByLabelText(es.form.fields.widthM), '1,4')
    await user.type(screen.getByLabelText(es.form.fields.nominalLengthM), '20')
    await user.type(screen.getByLabelText(es.form.fields.pricePerMeter), '15')
    await user.type(screen.getByLabelText(es.form.fields.pricePerRoll), '280')
    await user.type(screen.getByLabelText(t.rollLength), '19,8')
    await user.type(screen.getByLabelText(t.rollCost), '280')

    await user.click(register())

    expect(await screen.findByText(es.fieldErrors.already_exists)).toBeInTheDocument()
    expect(screen.getByLabelText(es.form.fields.sku)).toHaveFocus()
  })
})

describe('quantity of rolls', () => {
  const countField = () => screen.getByLabelText(t.rollCount)

  it('starts with one roll and creates the rows when a quantity is typed', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    expect(countField()).toHaveValue('1')
    await user.type(screen.getByLabelText(t.rollLength), '19,8')
    await user.type(screen.getByLabelText(t.rollCost), '280')

    await user.clear(countField())
    await user.type(countField(), '3')

    expect(screen.getAllByLabelText(t.rollLength)).toHaveLength(3)
    for (const field of screen.getAllByLabelText(t.rollLength)) expect(field).toHaveValue('19,8')
    for (const field of screen.getAllByLabelText(t.rollCost)) expect(field).toHaveValue('280')
  })

  it('lets the field be emptied while typing and shows the real number afterwards', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    await user.type(countField(), '{Backspace}')
    expect(countField()).toHaveValue('')
    expect(screen.getAllByLabelText(t.rollLength)).toHaveLength(1)

    await user.type(countField(), '2')
    await user.tab()
    await user.click(screen.getByRole('button', { name: t.addRoll }))

    expect(countField()).toHaveValue('3')
  })

  it('removing a roll with its button updates the quantity', async () => {
    const user = userEvent.setup()
    renderForm()
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    await user.clear(countField())
    await user.type(countField(), '3')
    await user.tab()

    await user.click(screen.getByRole('button', { name: t.removeRoll(1, 3) }))

    expect(countField()).toHaveValue('2')
  })

  it('submits one roll per row', async () => {
    const user = userEvent.setup()
    const onSubmit = renderForm()
    await user.click(screen.getByRole('button', { name: t.addPvc }))
    await user.selectOptions(screen.getByLabelText(t.productLabel), 'pvc-1')
    await user.type(screen.getByLabelText(t.rollLength), '20')
    await user.type(screen.getByLabelText(t.rollCost), '300')
    await user.clear(countField())
    await user.type(countField(), '2')

    await user.click(register())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const line = vi.mocked(onSubmit).mock.calls[0]![0].lines[0]!
    expect('rolls' in line && line.rolls).toEqual([
      { length_m: '20.00', cost: '300.00' },
      { length_m: '20.00', cost: '300.00' },
    ])
  })
})

