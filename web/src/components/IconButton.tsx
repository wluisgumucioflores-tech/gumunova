import type { ButtonHTMLAttributes, ReactNode } from 'react'
import { Link } from 'react-router'

// Icon-only actions. The icon replaces the visible text, so the action keeps an accessible name
// (aria-label) and a tooltip (title) with the same words.

interface IconActionProps {
  /** What the action does, in Spanish. Read by screen readers and shown as a tooltip. */
  label: string
  icon: ReactNode
  variant?: 'default' | 'danger'
}

const classNameFor = (variant: IconActionProps['variant']) =>
  variant === 'danger' ? 'button button-icon button-danger' : 'button button-icon'

export function IconButton({
  label,
  icon,
  variant,
  type = 'button',
  ...rest
}: IconActionProps & Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children' | 'className' | 'aria-label' | 'title'>) {
  return (
    <button {...rest} type={type} className={classNameFor(variant)} aria-label={label} title={label}>
      {icon}
    </button>
  )
}

export function IconLink({ label, icon, variant, to }: IconActionProps & { to: string }) {
  return (
    <Link to={to} className={classNameFor(variant)} aria-label={label} title={label}>
      {icon}
    </Link>
  )
}
