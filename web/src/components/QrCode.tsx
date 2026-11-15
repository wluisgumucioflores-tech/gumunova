import { useMemo } from 'react'
import QRCode from 'qrcode'

interface QrCodeProps {
  /** The text the code carries. */
  value: string
  /** What the code stands for, read by assistive technology. */
  label: string
}

const QUIET_ZONE = 2

/** A QR code drawn as one SVG path (black on white, so any scanner reads it, in either theme). */
export function QrCode({ value, label }: QrCodeProps) {
  const { size, path } = useMemo(() => {
    const { size, data } = QRCode.create(value, { errorCorrectionLevel: 'M' }).modules
    let path = ''
    for (let row = 0; row < size; row += 1) {
      for (let column = 0; column < size; column += 1) {
        if (data[row * size + column]) path += `M${column} ${row}h1v1h-1z`
      }
    }
    return { size, path }
  }, [value])

  const extent = size + QUIET_ZONE * 2
  return (
    <svg
      className="code-image"
      viewBox={`${-QUIET_ZONE} ${-QUIET_ZONE} ${extent} ${extent}`}
      shapeRendering="crispEdges"
      role="img"
      aria-label={label}
    >
      <rect x={-QUIET_ZONE} y={-QUIET_ZONE} width={extent} height={extent} fill="#fff" />
      <path d={path} fill="#000" />
    </svg>
  )
}
