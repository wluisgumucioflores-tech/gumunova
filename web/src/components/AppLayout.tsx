import { useEffect, useState } from 'react'
import { NavLink, Outlet } from 'react-router'
import { es } from '../i18n/es'
import { IconButton } from './IconButton'
import { BoxIcon, CartIcon, CloseIcon, GridIcon, ListIcon, MenuIcon, ScanIcon, UsersIcon } from './icons'

const NAV_ITEMS = [
  { to: '/catalogo', label: es.app.nav.catalog, icon: <GridIcon /> },
  { to: '/proveedores', label: es.app.nav.suppliers, icon: <UsersIcon /> },
  { to: '/compras', label: es.app.nav.purchases, icon: <BoxIcon /> },
  { to: '/ventas', label: es.app.nav.sales, icon: <CartIcon /> },
  { to: '/inventario', label: es.app.nav.inventory, icon: <ListIcon /> },
  { to: '/escanear', label: es.app.nav.scan, icon: <ScanIcon /> },
]

export function AppLayout() {
  // Only meaningful on narrow screens, where the sidebar is a drawer; on wide screens it is always visible.
  const [drawerOpen, setDrawerOpen] = useState(false)
  const closeDrawer = () => setDrawerOpen(false)

  useEffect(() => {
    if (!drawerOpen) {
      return
    }
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setDrawerOpen(false)
      }
    }
    document.addEventListener('keydown', closeOnEscape)
    return () => document.removeEventListener('keydown', closeOnEscape)
  }, [drawerOpen])

  return (
    <div className="app-shell">
      <a href="#main-content" className="skip-link">
        {es.app.skipToContent}
      </a>
      <header className="topbar">
        <IconButton label={es.app.openMenu} icon={<MenuIcon />} onClick={() => setDrawerOpen(true)} />
        <img className="brand-logo" src="/logo.jpg" alt={es.app.name} />
      </header>
      <button
        type="button"
        className="drawer-backdrop"
        data-open={drawerOpen}
        aria-label={es.app.closeMenu}
        tabIndex={-1}
        onClick={closeDrawer}
      />
      <aside className="sidebar" data-open={drawerOpen}>
        <div className="brand">
          <img className="brand-logo" src="/logo.jpg" alt={es.app.name} />
          <IconButton label={es.app.closeMenu} icon={<CloseIcon />} onClick={closeDrawer} />
        </div>
        <nav aria-label={es.app.navigationLabel}>
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              onClick={closeDrawer}
              className={({ isActive }) => (isActive ? 'nav-link is-active' : 'nav-link')}
            >
              {item.icon}
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <main id="main-content" className="content">
        <Outlet />
      </main>
    </div>
  )
}
