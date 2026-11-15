import { NavLink, Outlet } from 'react-router'
import { es } from '../../i18n/es'

const t = es.inventory

/** Header and section tabs shared by the inventory screens. */
export function InventoryLayout() {
  const tabClass = ({ isActive }: { isActive: boolean }) => (isActive ? 'tab is-active' : 'tab')
  return (
    <>
      <header className="page-header">
        <div>
          <h1>{t.title}</h1>
          <p className="muted">{t.subtitle}</p>
        </div>
      </header>
      <nav className="tabs" aria-label={t.tabsLabel}>
        <NavLink to="/inventario" end className={tabClass}>
          {t.tabs.stock}
        </NavLink>
        <NavLink to="/inventario/rollos" className={tabClass}>
          {t.tabs.rolls}
        </NavLink>
        <NavLink to="/inventario/movimientos" className={tabClass}>
          {t.tabs.movements}
        </NavLink>
      </nav>
      <Outlet />
    </>
  )
}
