# Web — panel admin

Aplicación Vite + React + TypeScript. Es el panel admin (dashboard); la landing page está en espera.
Consume la API Go bajo `/api/v1`; en desarrollo Vite la reenvía a `http://127.0.0.1:8080`.

Desde la raíz del repositorio:

- `make web` — servidor de desarrollo (`http://localhost:5173`).
- `make web-test` — pruebas (Vitest + Testing Library).
- `make web-build` — comprobación de tipos y build de producción.

Reglas del proyecto: ver `CLAUDE.md` (sección 5.1). Todo texto visible está en `src/i18n/es.ts`.

En contenedor (desde la raíz): `make app-up` compila la web con `docker/nginx/Dockerfile` (el compose está en `docker/docker-compose.yml`) y la sirve con nginx en `http://127.0.0.1:8081`; nginx reenvía `/api` a la API. `VITE_PUBLIC_BASE_URL` se pasa como argumento de compilación (público, nunca un secreto).
