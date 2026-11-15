import { useEffect, useRef, useState } from 'react'
import { isAbortError, messageForApiError } from '../api/client'

interface Answer<T> {
  key: string
  data: T | null
  error: string | null
}

/**
 * Loads data for the current filters. `key` identifies those filters: a new key starts a new
 * request and cancels the previous one. "Loading" is derived from whether the last answer
 * belongs to the current key, so no state is set while a request starts.
 */
export function useRemoteList<T>(key: string, load: (signal: AbortSignal) => Promise<T>) {
  const [answer, setAnswer] = useState<Answer<T> | null>(null)
  const [reloadToken, setReloadToken] = useState(0)
  const requestKey = `${key}#${reloadToken}`

  // The effect depends on the key only; it always calls the latest loader.
  const latestLoad = useRef(load)
  useEffect(() => {
    latestLoad.current = load
  })

  useEffect(() => {
    const controller = new AbortController()
    latestLoad
      .current(controller.signal)
      .then((data) => setAnswer({ key: requestKey, data, error: null }))
      .catch((error: unknown) => {
        if (isAbortError(error)) return
        setAnswer({ key: requestKey, data: null, error: messageForApiError(error) })
      })
    return () => controller.abort()
  }, [requestKey])

  const current = answer?.key === requestKey
  return {
    /** The last successful answer, kept while the next one loads. */
    data: answer?.data ?? null,
    loading: !current,
    error: current ? answer.error : null,
    reload: () => setReloadToken((token) => token + 1),
  }
}
