import { act, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../../api/products'
import { listOf, pvcProduct } from '../../test/fixtures'
import { ProductListPage } from './ProductListPage'

vi.mock('../../api/products')

function LocationState() {
  return <output data-testid="location-state">{JSON.stringify(useLocation().state)}</output>
}

function renderList(state?: { text: string; tone: 'success' | 'warning' }) {
  return render(
    <MemoryRouter initialEntries={[{ pathname: '/catalogo', state }]}>
      <Routes>
        <Route
          path="/catalogo"
          element={
            <>
              <ProductListPage />
              <LocationState />
            </>
          }
        />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.resetAllMocks()
  vi.useFakeTimers({ shouldAdvanceTime: true })
  vi.mocked(api.listProducts).mockResolvedValue(listOf([pvcProduct()]))
})

afterEach(() => {
  vi.useRealTimers()
})

describe('ProductListPage toast', () => {
  it('shows the message as a notification and removes it after 2 seconds', async () => {
    renderList({ text: 'Producto creado.', tone: 'success' })

    expect(await screen.findByText('PVC-FLORAL-20')).toBeInTheDocument()
    expect(screen.getByText('Producto creado.')).toBeInTheDocument()

    act(() => vi.advanceTimersByTime(2000))

    expect(screen.queryByText('Producto creado.')).not.toBeInTheDocument()
  })

  it('removes the message from the history entry so a refresh does not show it again', async () => {
    renderList({ text: 'Producto creado.', tone: 'success' })
    await screen.findByText('PVC-FLORAL-20')

    expect(screen.getByTestId('location-state')).toHaveTextContent('null')
  })

  it('shows nothing when the user arrives without a message', async () => {
    renderList()
    await screen.findByText('PVC-FLORAL-20')

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByText('Producto creado.')).not.toBeInTheDocument()
  })
})
