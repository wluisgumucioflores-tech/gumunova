import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Toast } from './Toast'
import { TOAST_DURATION_MS } from './toastConfig'

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('Toast', () => {
  it('shows the text as a polite status message', () => {
    render(<Toast text="Producto creado." tone="success" onDismiss={vi.fn()} />)

    expect(screen.getByRole('status')).toHaveTextContent('Producto creado.')
  })

  it('dismisses itself after 2 seconds, not before', () => {
    const onDismiss = vi.fn()
    render(<Toast text="Producto creado." tone="success" onDismiss={onDismiss} />)

    act(() => vi.advanceTimersByTime(1999))
    expect(onDismiss).not.toHaveBeenCalled()

    act(() => vi.advanceTimersByTime(1))
    expect(onDismiss).toHaveBeenCalledTimes(1)
    expect(TOAST_DURATION_MS.success).toBe(2000)
  })

  it('announces warnings as alerts and keeps them longer so they can be read', () => {
    const onDismiss = vi.fn()
    render(<Toast text="No se pudo subir la imagen." tone="warning" onDismiss={onDismiss} />)

    expect(screen.getByRole('alert')).toHaveClass('toast-warning')
    act(() => vi.advanceTimersByTime(TOAST_DURATION_MS.success))
    expect(onDismiss).not.toHaveBeenCalled()
    act(() => vi.advanceTimersByTime(TOAST_DURATION_MS.warning - TOAST_DURATION_MS.success))
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('does not call onDismiss when removed before the time is up', () => {
    const onDismiss = vi.fn()
    const { unmount } = render(<Toast text="Producto creado." tone="success" onDismiss={onDismiss} />)

    unmount()
    act(() => vi.advanceTimersByTime(5000))

    expect(onDismiss).not.toHaveBeenCalled()
  })
})
