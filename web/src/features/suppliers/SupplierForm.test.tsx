import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../api/client'
import type { SupplierInput } from '../../api/suppliers'
import { es } from '../../i18n/es'
import { supplier } from '../../test/fixtures'
import { SupplierForm } from './SupplierForm'

const f = es.suppliers.fields
const save = () => screen.getByRole('button', { name: es.common.save })

describe('SupplierForm', () => {
  it('requires the name and does not submit an incomplete form', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<SupplierForm onSubmit={onSubmit} onCancel={() => {}} />)

    await user.click(save())

    expect(onSubmit).not.toHaveBeenCalled()
    expect(screen.getByRole('alert')).toHaveTextContent(es.suppliers.fixErrors)
    expect(screen.getByText(es.fieldErrors.required)).toBeInTheDocument()
    expect(screen.getByLabelText(f.name)).toHaveFocus()
    expect(screen.getByLabelText(f.name)).toHaveAttribute('aria-invalid', 'true')
  })

  it('marks phone, email and notes as optional', () => {
    render(<SupplierForm onSubmit={vi.fn()} onCancel={() => {}} />)

    expect(screen.getAllByText(es.common.optional)).toHaveLength(3)
  })

  it('submits the trimmed values', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: SupplierInput) => Promise<void>>().mockResolvedValue()
    render(<SupplierForm onSubmit={onSubmit} onCancel={() => {}} />)

    await user.type(screen.getByLabelText(f.name), '  Plásticos del Sur ')
    await user.type(screen.getByLabelText(new RegExp(f.phone)), '700 00000')
    await user.type(screen.getByLabelText(new RegExp(f.email)), 'ventas@sur.com')
    await user.type(screen.getByLabelText(new RegExp(f.notes)), 'Entrega en 3 días')
    await user.click(save())

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit).toHaveBeenCalledWith({
      name: 'Plásticos del Sur',
      phone: '700 00000',
      email: 'ventas@sur.com',
      notes: 'Entrega en 3 días',
    })
  })

  it('rejects a malformed email in Spanish', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    render(<SupplierForm onSubmit={onSubmit} onCancel={() => {}} />)

    await user.type(screen.getByLabelText(f.name), 'Proveedor')
    await user.type(screen.getByLabelText(new RegExp(f.email)), 'no-es-correo')
    await user.click(save())

    expect(onSubmit).not.toHaveBeenCalled()
    expect(screen.getByText(es.fieldErrors.invalid_email)).toBeInTheDocument()
    expect(screen.getByLabelText(new RegExp(f.email))).toHaveFocus()
  })

  it('shows server-side field errors in Spanish', async () => {
    const user = userEvent.setup()
    const onSubmit = vi
      .fn<(input: SupplierInput) => Promise<void>>()
      .mockRejectedValue(new ApiError(422, 'validation_failed', [{ field: 'email', code: 'invalid_email' }]))
    render(<SupplierForm onSubmit={onSubmit} onCancel={() => {}} />)

    await user.type(screen.getByLabelText(f.name), 'Proveedor')
    await user.click(save())

    expect(await screen.findByText(es.fieldErrors.invalid_email)).toBeInTheDocument()
  })

  it('shows a Spanish message when the server cannot be reached', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn<(input: SupplierInput) => Promise<void>>().mockRejectedValue(new ApiError(0, 'network'))
    render(<SupplierForm onSubmit={onSubmit} onCancel={() => {}} />)

    await user.type(screen.getByLabelText(f.name), 'Proveedor')
    await user.click(save())

    expect(await screen.findByRole('alert')).toHaveTextContent(es.apiErrors.network)
  })

  it('blocks a second submit while the first one is in flight', async () => {
    const user = userEvent.setup()
    let finish: () => void = () => {}
    const onSubmit = vi.fn<(input: SupplierInput) => Promise<void>>(
      () => new Promise<void>((resolve) => (finish = resolve)),
    )
    render(<SupplierForm onSubmit={onSubmit} onCancel={() => {}} />)

    await user.type(screen.getByLabelText(f.name), 'Proveedor')
    await user.click(save())
    const busy = await screen.findByRole('button', { name: es.common.saving })
    expect(busy).toBeDisabled()
    await user.click(busy)

    expect(onSubmit).toHaveBeenCalledTimes(1)
    finish()
    await waitFor(() => expect(save()).toBeEnabled())
  })

  it('loads the supplier when editing', () => {
    render(<SupplierForm supplier={supplier()} onSubmit={vi.fn()} onCancel={() => {}} />)

    expect(screen.getByLabelText(f.name)).toHaveValue('Plásticos del Sur')
    expect(screen.getByLabelText(new RegExp(f.email))).toHaveValue('ventas@sur.com')
    expect(screen.getByLabelText(new RegExp(f.notes))).toHaveValue('Entrega en 3 días')
  })

  it('calls onCancel', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()
    render(<SupplierForm onSubmit={vi.fn()} onCancel={onCancel} />)

    await user.click(screen.getByRole('button', { name: es.common.cancel }))

    expect(onCancel).toHaveBeenCalled()
  })
})
