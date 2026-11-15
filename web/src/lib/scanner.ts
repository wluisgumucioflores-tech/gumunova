import jsQR from 'jsqr'

// Reading codes from images. Browsers that ship the BarcodeDetector API read QR and Code 128
// natively; the others fall back to jsQR, which reads QR codes only. Anything it reads is just
// text: the server decides what, if anything, it identifies.

interface DetectedCode {
  rawValue: string
}
interface BarcodeDetectorLike {
  detect(source: ImageBitmapSource): Promise<DetectedCode[]>
}
type BarcodeDetectorConstructor = new (options?: { formats?: string[] }) => BarcodeDetectorLike

const FORMATS = ['qr_code', 'code_128']

function nativeDetector(): BarcodeDetectorLike | null {
  const Detector = (globalThis as { BarcodeDetector?: BarcodeDetectorConstructor }).BarcodeDetector
  if (!Detector) return null
  try {
    return new Detector({ formats: FORMATS })
  } catch {
    return null // the browser knows the API but not these formats
  }
}

/** Reads a QR code out of raw pixels (RGBA). Returns null when there is none. */
export function decodeQrPixels(image: ImageData): string | null {
  return jsQR(image.data, image.width, image.height, { inversionAttempts: 'attemptBoth' })?.data ?? null
}

/**
 * A function that reads the code in a video frame, a canvas or a bitmap, or null when there is
 * none. It uses the native detector when the browser has one.
 */
export function createCodeReader(): (source: HTMLVideoElement | HTMLCanvasElement | ImageBitmap) => Promise<string | null> {
  const detector = nativeDetector()
  let canvas: HTMLCanvasElement | null = null

  return async (source) => {
    if (detector) {
      try {
        return (await detector.detect(source))[0]?.rawValue ?? null
      } catch {
        return null // a frame the detector could not read
      }
    }
    // Fallback: copy the frame (scaled down) to a canvas and read its pixels.
    const width = source instanceof HTMLVideoElement ? source.videoWidth : source.width
    const height = source instanceof HTMLVideoElement ? source.videoHeight : source.height
    if (!width || !height) return null
    const scale = Math.min(1, 800 / Math.max(width, height))
    canvas ??= document.createElement('canvas')
    canvas.width = Math.round(width * scale)
    canvas.height = Math.round(height * scale)
    const context = canvas.getContext('2d', { willReadFrequently: true })
    if (!context) return null
    context.drawImage(source, 0, 0, canvas.width, canvas.height)
    return decodeQrPixels(context.getImageData(0, 0, canvas.width, canvas.height))
  }
}

/** Reads the code in a photo the person took. Returns null when no code is found. */
export async function readCodeFromImageFile(file: File): Promise<string | null> {
  const bitmap = await createImageBitmap(file)
  try {
    return await createCodeReader()(bitmap)
  } finally {
    bitmap.close()
  }
}

/** Whether this page may ask for the camera: browsers only allow it on https or localhost. */
export function cameraAvailable(): boolean {
  return typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia && globalThis.isSecureContext !== false
}
