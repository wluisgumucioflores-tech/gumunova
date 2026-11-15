import QRCode from 'qrcode'
import { describe, expect, it } from 'vitest'
import { decodeQrPixels } from './scanner'

/** Draws a QR code as RGBA pixels, like a camera frame of a printed label (black on white, with a quiet zone). */
function qrPixels(text: string, scale = 6, quiet = 4, dark = [0, 0, 0], light = [255, 255, 255]) {
  const { size, data } = QRCode.create(text, { errorCorrectionLevel: 'M' }).modules
  const side = (size + quiet * 2) * scale
  const pixels = new Uint8ClampedArray(side * side * 4)
  for (let y = 0; y < side; y += 1) {
    for (let x = 0; x < side; x += 1) {
      const column = Math.floor(x / scale) - quiet
      const row = Math.floor(y / scale) - quiet
      const on = column >= 0 && row >= 0 && column < size && row < size && data[row * size + column]
      const color = on ? dark : light
      pixels.set([color[0]!, color[1]!, color[2]!, 255], (y * side + x) * 4)
    }
  }
  return { data: pixels, width: side, height: side } as ImageData
}

describe('decodeQrPixels', () => {
  it('reads the code printed on a label', () => {
    expect(decodeQrPixels(qrPixels('P-1A2B3C4D5E6F'))).toBe('P-1A2B3C4D5E6F')
    expect(decodeQrPixels(qrPixels('R-0123456789AB', 3))).toBe('R-0123456789AB')
  })

  it('reads a light code on a dark background too', () => {
    expect(decodeQrPixels(qrPixels('P-FFFFFFFFFFFF', 6, 4, [255, 255, 255], [0, 0, 0]))).toBe('P-FFFFFFFFFFFF')
  })

  it('returns null when the picture has no code', () => {
    const blank = { data: new Uint8ClampedArray(100 * 100 * 4).fill(255), width: 100, height: 100 } as ImageData
    expect(decodeQrPixels(blank)).toBeNull()
  })
})
