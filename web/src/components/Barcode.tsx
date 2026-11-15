import { useLayoutEffect, useRef } from 'react'
import JsBarcode from 'jsbarcode'

interface BarcodeProps {
  /** The text the code carries. Code 128 reads any plain ASCII, which covers every SKU. */
  value: string
  label: string
}

/** A Code 128 barcode (black on white). The text under it is shown by the caller, not drawn here. */
export function Barcode({ value, label }: BarcodeProps) {
  const svgRef = useRef<SVGSVGElement>(null)

  useLayoutEffect(() => {
    const svg = svgRef.current
    if (!svg) return
    try {
      JsBarcode(svg, value, { format: 'CODE128', displayValue: false, margin: 8, height: 60, width: 2, background: '#fff', lineColor: '#000' })
    } catch {
      svg.replaceChildren() // a value Code 128 cannot carry leaves the image empty; the caller still prints the text
      return
    }
    // Let the image scale with its label instead of keeping the pixel size the library chose.
    const width = svg.getAttribute('width')
    const height = svg.getAttribute('height')
    if (width && height) svg.setAttribute('viewBox', `0 0 ${parseFloat(width)} ${parseFloat(height)}`)
    svg.removeAttribute('width')
    svg.removeAttribute('height')
  }, [value])

  return <svg ref={svgRef} className="code-image" role="img" aria-label={label} />
}
