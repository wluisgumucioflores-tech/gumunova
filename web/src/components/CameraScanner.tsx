import { useEffect, useRef } from 'react'
import { createCodeReader } from '../lib/scanner'
import { es } from '../i18n/es'

const SCAN_INTERVAL_MS = 150

interface CameraScannerProps {
  /** Called with the text of the first code read. The camera keeps running until the parent unmounts it. */
  onCode: (text: string) => void
  /** Called when the camera cannot start (permission denied, no camera, ...). */
  onError: () => void
}

/** Shows the rear camera and reads codes from it, a few times per second. */
export function CameraScanner({ onCode, onError }: CameraScannerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  // The latest callbacks, so the camera is not restarted when the parent re-renders.
  const callbacks = useRef({ onCode, onError })
  useEffect(() => {
    callbacks.current = { onCode, onError }
  })

  useEffect(() => {
    let stopped = false
    let timer: ReturnType<typeof setTimeout> | undefined
    let stream: MediaStream | undefined
    const read = createCodeReader()

    async function start() {
      try {
        stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: { ideal: 'environment' } }, audio: false })
      } catch {
        if (!stopped) callbacks.current.onError()
        return
      }
      const video = videoRef.current
      if (stopped || !video) {
        stream.getTracks().forEach((track) => track.stop())
        return
      }
      video.srcObject = stream
      await video.play().catch(() => undefined)
      tick()
    }

    async function tick() {
      const video = videoRef.current
      if (stopped || !video) return
      if (video.readyState >= 2) {
        const text = await read(video)
        if (stopped) return
        if (text) {
          callbacks.current.onCode(text)
          return // the parent decides whether to keep scanning by keeping this mounted
        }
      }
      timer = setTimeout(tick, SCAN_INTERVAL_MS)
    }

    void start()
    return () => {
      stopped = true
      clearTimeout(timer)
      stream?.getTracks().forEach((track) => track.stop())
    }
  }, [])

  return (
    <div className="scanner">
      <video ref={videoRef} className="scanner-video" muted playsInline aria-label={es.scan.cameraLabel} />
      <div className="scanner-frame" aria-hidden="true" />
    </div>
  )
}
