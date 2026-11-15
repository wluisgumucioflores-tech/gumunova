import { useCallback, useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router'
import type { ToastTone } from '../components/toastConfig'

interface ToastMessage {
  text: string
  tone: ToastTone
}

/**
 * A save on a form screen sends the user back to a list with a message. This returns it once,
 * to be shown as a toast (`showToast` shows one raised on the same screen), and removes it from the history entry so a refresh or "back" does not
 * show it again.
 */
export function useNavigationToast() {
  const location = useLocation()
  const navigate = useNavigate()
  const incoming = location.state as { text?: string; tone?: ToastTone } | null

  const [toast, setToast] = useState<ToastMessage | null>(
    incoming?.text ? { text: incoming.text, tone: incoming.tone ?? 'success' } : null,
  )
  const dismissToast = useCallback(() => setToast(null), [])
  const showToast = useCallback((text: string, tone: ToastTone = 'success') => setToast({ text, tone }), [])

  const hasIncoming = incoming !== null
  useEffect(() => {
    if (hasIncoming) navigate(location.pathname + location.search, { replace: true, state: null })
  }, [hasIncoming, navigate, location.pathname, location.search])

  return { toast, dismissToast, showToast }
}
