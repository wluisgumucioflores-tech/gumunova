import { useEffect } from 'react'
import { TOAST_DURATION_MS, type ToastTone } from './toastConfig'

interface ToastProps {
  text: string
  tone: ToastTone
  /** Called once the toast has been visible for its duration. Keep it stable (useCallback). */
  onDismiss: () => void
}

/** A small floating notification that dismisses itself. */
export function Toast({ text, tone, onDismiss }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, TOAST_DURATION_MS[tone])
    return () => clearTimeout(timer)
  }, [text, tone, onDismiss])

  return (
    <div className="toast-region">
      <p className={`toast toast-${tone}`} role={tone === 'warning' ? 'alert' : 'status'}>
        {text}
      </p>
    </div>
  )
}
