import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import type { ProductInput } from '../../api/products'
import { es } from '../../i18n/es'
import { pvcProduct } from '../../test/fixtures'
import { ProductForm } from './ProductForm'

const f = es.form.fields

async function fillValidPvc(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(f.sku), 'pvc-floral-20')
  await user.type(screen.getByLabelText(f.name), 'PVC floral 20 m')
  await user.type(screen.getByLabelText(f.widthM), '1,4')
  await user.type(screen.getByLabelText(f.nominalLengthM), '20')
  await user.type(screen.getByLabelText(f.pricePerMeter), '15,5')
  await user.type(screen.getByLabelText(f.pricePerRoll), '280')
}

describe('ProductForm stock alert', () => {
  it('sends the minimum stock typed for a PVC product', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: ProductInput) => Promise<void>>().mockResolvedValue()
    render(<ProductForm onSubmit={onSubmit} onCancel={() => {}} />)
    await fillValidPvc(user)
    await user.type(screen.getByLabelText(f.minimumStockM, { exact: false }), '50')

    await user.click(screen.getByRole('button', { name: es.form.submit }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit.mock.calls[0]![0].pvc?.minimum_stock_m).toBe('50.00')
  })

  it('asks for whole units for a tablecloth and explains the alert', async () => {
    const user = userEvent.setup()
    render(<ProductForm onSubmit={vi.fn()} onCancel={() => {}} />)
    await user.click(screen.getByRole('radio', { name: es.catalog.types.tablecloth }))

    const field = screen.getByLabelText(f.minimumUnits, { exact: false })
    await user.type(field, '2,5')
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(field).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByText(f.minimumStockHint)).toBeInTheDocument()
  })
})

describe('ProductForm (create)', () => {
  it('shows Spanish errors and does not submit an incomplete form', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<ProductForm onSubmit={onSubmit} onCancel={() => {}} />)

    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(onSubmit).not.toHaveBeenCalled()
    expect(screen.getByRole('alert')).toHaveTextContent(es.form.fixErrors)
    expect(screen.getAllByText(es.fieldErrors.required).length).toBeGreaterThanOrEqual(6)
    expect(screen.getByLabelText(f.sku)).toHaveFocus()
    expect(screen.getByLabelText(f.sku)).toHaveAttribute('aria-invalid', 'true')
  })

  it('submits a PVC payload with canonical decimals', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: ProductInput) => Promise<void>>().mockResolvedValue()
    render(<ProductForm onSubmit={onSubmit} onCancel={() => {}} />)

    await fillValidPvc(user)
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit).toHaveBeenCalledWith({
      sku: 'pvc-floral-20',
      name: 'PVC floral 20 m',
      product_type: 'pvc',
      pvc: {
        material: '',
        design: '',
        color: '',
        texture: '',
        width_m: '1.40',
        nominal_length_m: '20.00',
        thickness_mm: null,
        reference_price_per_meter: '15.50',
        reference_price_per_roll: '280.00',
        minimum_stock_m: '0.00',
      },
    })
  })

  it('switches to tablecloth fields and submits a tablecloth payload', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: ProductInput) => Promise<void>>().mockResolvedValue()
    render(<ProductForm onSubmit={onSubmit} onCancel={() => {}} />)

    await user.click(screen.getByLabelText(es.catalog.types.tablecloth))

    expect(screen.queryByLabelText(f.widthM)).not.toBeInTheDocument()
    await user.type(screen.getByLabelText(f.sku), 'man-rojo-01')
    await user.type(screen.getByLabelText(f.name), 'Mantel rojo')
    await user.type(screen.getByLabelText(f.unitPrice), '60')
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    const input = onSubmit.mock.calls[0]?.[0]
    expect(input?.product_type).toBe('tablecloth')
    expect(input?.pvc).toBeUndefined()
    expect(input?.tablecloth?.unit_price).toBe('60.00')
  })

  it('shows server-side field errors in Spanish', async () => {
    const user = userEvent.setup()
    const onSubmit = vi
      .fn<(input: ProductInput) => Promise<void>>()
      .mockRejectedValue(new ApiError(422, 'validation_failed', [{ field: 'pvc.width_m', code: 'must_be_positive' }]))
    render(<ProductForm onSubmit={onSubmit} onCancel={() => {}} />)

    await fillValidPvc(user)
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(await screen.findByText(es.fieldErrors.must_be_positive)).toBeInTheDocument()
    expect(screen.getByLabelText(f.widthM)).toHaveAttribute('aria-invalid', 'true')
  })

  it('reports a duplicated SKU on the SKU field', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: ProductInput) => Promise<void>>().mockRejectedValue(new ApiError(409, 'sku_already_exists'))
    render(<ProductForm onSubmit={onSubmit} onCancel={() => {}} />)

    await fillValidPvc(user)
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(await screen.findByText(es.apiErrors.sku_already_exists)).toBeInTheDocument()
    expect(screen.getByLabelText(f.sku)).toHaveFocus()
  })

  it('shows a Spanish message when the server cannot be reached', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: ProductInput) => Promise<void>>().mockRejectedValue(new ApiError(0, 'network'))
    render(<ProductForm onSubmit={onSubmit} onCancel={() => {}} />)

    await fillValidPvc(user)
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
  })

  it('blocks a second submit while the first one is in flight', async () => {
    const user = userEvent.setup()
    let finish: () => void = () => {}
    const onSubmit = vi.fn<(input: ProductInput) => Promise<void>>(
      () => new Promise<void>((resolve) => (finish = resolve)),
    )
    render(<ProductForm onSubmit={onSubmit} onCancel={() => {}} />)

    await fillValidPvc(user)
    const button = screen.getByRole('button', { name: es.form.submit })
    await user.click(button)

    const busyButton = await screen.findByRole('button', { name: es.common.saving })
    expect(busyButton).toBeDisabled()
    await user.click(busyButton)
    expect(onSubmit).toHaveBeenCalledTimes(1)

    finish()
    await waitFor(() => expect(screen.getByRole('button', { name: es.form.submit })).toBeEnabled())
  })

  it('calls onCancel', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    render(<ProductForm onSubmit={vi.fn()} onCancel={onCancel} />)

    await user.click(screen.getByRole('button', { name: es.common.cancel }))

    expect(onCancel).toHaveBeenCalled()
  })
})

describe('ProductForm (edit)', () => {
  it('loads the product with Spanish decimals and locks SKU and type', () => {
    render(<ProductForm product={pvcProduct()} onSubmit={vi.fn()} onCancel={() => {}} />)

    expect(screen.getByLabelText(f.sku)).toHaveValue('PVC-FLORAL-20')
    expect(screen.getByLabelText(f.sku)).toHaveAttribute('readonly')
    expect(screen.getByLabelText(f.widthM)).toHaveValue('1,40')
    expect(screen.getByLabelText(f.pricePerMeter)).toHaveValue('15,50')
    expect(screen.queryByRole('radiogroup')).not.toBeInTheDocument()
    expect(screen.getByText(es.form.productTypeLocked)).toBeInTheDocument()
  })

  it('submits the edited values', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: ProductInput) => Promise<void>>().mockResolvedValue()
    render(<ProductForm product={pvcProduct()} onSubmit={onSubmit} onCancel={() => {}} />)

    const price = screen.getByLabelText(f.pricePerMeter)
    await user.clear(price)
    await user.type(price, '17,75')
    await user.click(screen.getByRole('button', { name: es.form.submit }))

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit.mock.calls[0]?.[0].pvc?.reference_price_per_meter).toBe('17.75')
  })
})
