import { es } from '../../i18n/es'

interface PaginationProps {
  summary: string
  offset: number
  pageSize: number
  total: number
  busy: boolean
  onChange: (offset: number) => void
}

export function Pagination({ summary, offset, pageSize, total, busy, onChange }: PaginationProps) {
  return (
    <footer className="pagination">
      <p className="muted" aria-live="polite">
        {summary}
      </p>
      <div className="pagination-buttons">
        <button type="button" className="button" disabled={offset === 0 || busy} onClick={() => onChange(Math.max(0, offset - pageSize))}>
          {es.common.previous}
        </button>
        <button type="button" className="button" disabled={offset + pageSize >= total || busy} onClick={() => onChange(offset + pageSize)}>
          {es.common.next}
        </button>
      </div>
    </footer>
  )
}
