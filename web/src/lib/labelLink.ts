// The QR of a label carries a web address, so any phone camera opens the scan page directly.
// Barcodes carry the bare code: a hardware scanner types it into the scan page.

const LABEL_PATH = '/producto'

/** The address printed QR codes open: the configured public address, or the one the app is open at. */
export function publicBaseUrl(): string {
  const configured = (import.meta.env.VITE_PUBLIC_BASE_URL as string | undefined)?.trim()
  return (configured || window.location.origin).replace(/\/+$/, '')
}

/** The address a label's QR carries: /producto/<label code>. It opens the page already showing that product. */
export function labelUrl(code: string, base: string = publicBaseUrl()): string {
  return `${base}${LABEL_PATH}/${encodeURIComponent(code)}`
}

/** Extracts the code from what a scanner read: the address of a label's QR, or a bare code. */
export function codeFromScan(text: string): string {
  const trimmed = text.trim()
  try {
    const match = new URL(trimmed).pathname.match(/^\/producto\/([^/]+)\/?$/)
    if (match?.[1]) return decodeURIComponent(match[1]).trim()
  } catch {
    // not an address, or not a label address: it is a bare code
  }
  return trimmed
}

/** True when the address only works on this computer, so a phone could not open a QR that points to it. */
export function isLocalAddress(base: string): boolean {
  try {
    const { hostname } = new URL(base)
    return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]' || hostname.endsWith('.localhost')
  } catch {
    return false
  }
}
