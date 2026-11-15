export type ToastTone = 'success' | 'warning'

/** Successes are brief. A warning carries more text the user needs to read, so it stays longer. */
export const TOAST_DURATION_MS: Record<ToastTone, number> = {
  success: 2000,
  warning: 6000,
}
