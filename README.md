# Sistema de inventario y ventas de PVC y manteles

> Nombre provisional del proyecto. Aplicación Android para administrar compras, inventario, ventas y reportes de dos líneas de productos: rollos de PVC para mesas y manteles terminados.

## 1. Objetivo

Centralizar el control operativo y económico del negocio, con trazabilidad por rollo de PVC y por producto terminado. El sistema debe registrar ventas de tres maneras —formulario, texto escrito y texto dictado mediante el micrófono del teclado Android— sin permitir que una interpretación automática modifique el inventario sin validación y confirmación.

La primera entrega se enfoca en una **aplicación web sencilla (Vite + React + TypeScript)** y un **backend en Go**. La aplicación Android queda para una etapa posterior y consumirá la misma API.

## 2. Tecnologías y arquitectura acordadas

| Área | Decisión |
|---|---|
| Aplicación web (primera entrega) | Vite + React + TypeScript, enfoque sencillo. Dos áreas: **panel admin (dashboard)**, que es el foco actual, y **landing page**, en espera |
| Aplicación móvil (etapa posterior) | Kotlin + Jetpack Compose |
| Arquitectura Android (etapa posterior) | Clean Architecture + MVVM + flujo unidireccional de datos (UDF) |
| Estado Android (etapa posterior) | ViewModel + StateFlow + Coroutines |
| Inyección de dependencias Android (etapa posterior) | Hilt |
| Backend | Go |
| Arquitectura backend | Hexagonal (Ports & Adapters) |
| API | REST sobre HTTPS |
| Base de datos principal | PostgreSQL, ejecutado en un contenedor Docker |
| Almacenamiento de archivos | Cloudflare R2 |
| Interpretación de ventas en lenguaje natural | DeepSeek API, invocada desde Go |
| Idioma del código | Inglés |
| Idioma visible de la aplicación | Español |

**Importante:** Cloudflare R2 es almacenamiento de objetos/archivos, no la base de datos transaccional. PostgreSQL será la fuente de verdad de productos, rollos, existencias, compras, ventas, pagos y movimientos. R2 se reservará para imágenes, comprobantes u otros archivos. Referencias: [Cloudflare R2](https://developers.cloudflare.com/r2/) y [PostgreSQL](https://www.postgresql.org/docs/current/).

## 3. Líneas de inventario

### 3.1 PVC en rollos

El PVC se compra en rollos y puede venderse por metro o como rollo completo.

Presentaciones iniciales identificadas:

- Rollo nominal de 20 metros × 1,40 metros de ancho.
- Rollo nominal de 30 metros × 1,40 metros de ancho.

La longitud real recibida debe poder registrarse; no se debe asumir que todos los rollos tienen exactamente la longitud nominal.

Cada variante comercial tendrá un **SKU de producto**. Cada rollo físico comprado tendrá además un **código único de rollo**. Varios rollos pueden compartir SKU, pero nunca deben compartir el identificador físico.

Datos previstos de la variante/SKU:

- SKU y nombre comercial.
- Categoría y tipo de material.
- Diseño/modelo, color, patrón y textura.
- Ancho, longitud nominal y espesor (cuando se conozca).
- Unidad de venta y precios de referencia por metro y por rollo completo.
- Imagen de referencia opcional.

Datos previstos del rollo físico:

- Código único (por ejemplo, `ROL-000125`) y SKU relacionado.
- Longitud inicial real y metros disponibles.
- Costo de compra y gastos atribuibles, si existen.
- Costo por metro calculado a partir de la longitud utilizable inicial.
- Proveedor (opcional), fecha y documento de compra.
- Historial de cortes, ventas, daños, devoluciones y ajustes.
- Código QR o código de barras opcional.

### 3.2 Manteles terminados

Los manteles se compran terminados y se revenden por unidad. Tendrán SKU propio, descripción, diseño, dimensiones, color, proveedor, costo unitario, precio de venta y cantidad disponible. No se mezclarán con el inventario de PVC.

## 4. Funcionalidades previstas

| Módulo | Funcionalidades |
|---|---|
| Inicio / dashboard | Resumen de ventas, ganancia bruta, rollos, metros y unidades disponibles; accesos rápidos. |
| Escanear etiquetas | Leer el QR o código de barras de una etiqueta (cámara, foto o lector) y ver el nombre, el tipo y el precio según sea PVC o mantel. |
| Catálogo | Crear y consultar SKU de PVC y manteles, atributos, imágenes y precios de referencia. Imprimir etiquetas con código QR o de barras (código único y regenerable de cada producto) de todos los productos o de los seleccionados. |
| Inventario PVC | Registrar rollos individuales, longitudes reales, costos, metros disponibles, cortes, daños y movimientos. |
| Inventario de manteles | Control de existencias por unidad y stock mínimo. |
| Compras | Registrar proveedor, fecha, comprobante, líneas de compra, costos y gastos adicionales. Una compra de PVC crea un registro físico independiente por cada rollo recibido. Un producto nuevo puede crearse desde la propia compra. Se puede editar mientras nada de ella se haya vendido (el inventario se ajusta con movimientos de corrección). |
| Ventas | Registrar por UI, texto escrito o dictado del teclado; vender PVC por metro o rollo completo y manteles por unidad; descuentos y precio particular cuando corresponda. |
| Cobros | Registrar efectivo, QR, transferencia u otros métodos; pagos completos, parciales y saldos pendientes. |
| Clientes | Datos básicos opcionales para asociar ventas y saldos por cobrar. |
| Proveedores | Datos básicos e historial de compras. |
| Movimientos | Historial auditable de entradas, salidas, cortes, devoluciones, daños, ajustes y anulaciones. |
| Gastos | Registrar gastos operativos como transporte u otros conceptos. |
| Reportes | Ventas, compras, costos, ganancias, inventario, rentabilidad, movimientos y pendientes; filtros por fechas y producto; exportación a Excel/PDF prevista. |
| IA | Interpretar una descripción de venta y devolver un borrador estructurado que el usuario revisa antes de confirmar. |

## 5. Modalidades de registro de venta

1. **Manual (UI):** seleccionar producto, modalidad, rollo si aplica, cantidad, precio, descuentos y pago.
2. **Texto:** escribir una instrucción en español, por ejemplo: “Vendí 3 metros de PVC floral a 15 bolivianos el metro”.
3. **Dictado:** pulsar el icono de micrófono de la pantalla para enfocar el campo de texto y utilizar el dictado del teclado del teléfono (Gboard u otro IME). La aplicación procesa el texto resultante; no necesita grabar ni enviar audio en esta primera versión.

Para texto y dictado, el flujo será: texto → backend Go → DeepSeek → borrador estructurado → validación de negocio → vista previa en Android → confirmación explícita → registro transaccional. La disponibilidad del dictado depende del teclado, idioma y configuración del dispositivo; la aplicación no debe prometer que puede activar el micrófono interno de cualquier teclado. En la web, el dictado está pendiente de decisión (ver `docs/09-open-decisions.md`); la entrada escrita siempre estará disponible.

## 6. Reportes previstos

- Ventas por día, semana, mes y rango personalizado.
- Ingresos, costo de ventas y ganancia bruta.
- Gastos operativos y resultado neto operativo.
- Ventas separadas por PVC y manteles.
- Metros vendidos, rollos completos vendidos y unidades de manteles vendidas.
- Rentabilidad por producto, SKU y rollo.
- Compras e inversión por proveedor y período.
- Existencias: rollos, metros disponibles y unidades de manteles.
- Metros dañados/desperdiciados y otros ajustes.
- Historial de movimientos y cuentas pendientes de cobro.
- Exportación de reportes a Excel y PDF.

Definiciones financieras importantes:

- **Ventas netas:** ventas confirmadas menos descuentos y devoluciones aplicables.
- **Costo de ventas (COGS):** costo asignado a los artículos efectivamente vendidos.
- **Ganancia bruta:** ventas netas − costo de ventas.
- **Resultado neto operativo:** ganancia bruta − gastos operativos registrados.
- Las compras de inventario no se deben restar íntegramente de la ganancia del período; aumentan el inventario y su costo se reconoce como costo de ventas cuando el producto se vende. Los reportes también mostrarán compras/inversión por separado.

## 7. Reglas esenciales

- SKU identifica una variante comercial; el código de rollo identifica una unidad física específica.
- Un rollo conserva su propio costo, longitud inicial y saldo de metros.
- Se admiten cantidades decimales de metros; no usar tipos de punto flotante para dinero ni cálculos monetarios.
- Una venta por metro descuenta metros de un rollo específico.
- Una venta como rollo completo solo procede cuando el rollo está íntegro y no ha sido cortado ni parcialmente vendido. Los remanentes se venden por metro.
- Cada cambio de existencia genera un movimiento auditable; no modificar saldos sin registrar el motivo.
- Compras, ventas, pagos y movimientos relacionados deben persistirse con transacciones de base de datos.
- Una operación confirmada no se elimina físicamente: se anula o revierte mediante movimientos compensatorios.
- La IA solo propone datos; nunca decide el rollo, confirma la venta ni modifica el inventario por sí misma.
- El backend Go valida disponibilidad, precio, importes, permisos y reglas antes de confirmar cualquier operación.
- No exponer credenciales de PostgreSQL, R2 ni DeepSeek en la aplicación Android.

## 8. Estructura documental

- [`CLAUDE.md`](./CLAUDE.md): instrucciones para agentes de código y reglas obligatorias del repositorio.
- [`docs/01-product-overview.md`](./docs/01-product-overview.md): visión, alcance y usuarios.
- [`docs/02-functional-scope.md`](./docs/02-functional-scope.md): funcionalidades y flujos por módulo.
- [`docs/03-business-rules.md`](./docs/03-business-rules.md): reglas de inventario, compras, ventas y cálculos.
- [`docs/04-architecture.md`](./docs/04-architecture.md): arquitectura Go y Android, límites y estructura sugerida.
- [`docs/05-domain-model.md`](./docs/05-domain-model.md): entidades y relaciones conceptuales.
- [`docs/06-ai-assisted-sales.md`](./docs/06-ai-assisted-sales.md): interpretación de ventas por texto/dictado y controles.
- [`docs/07-ui-and-localization.md`](./docs/07-ui-and-localization.md): navegación, pantallas y regla inglés/español.
- [`docs/08-reporting.md`](./docs/08-reporting.md): indicadores, fórmulas y filtros.
- [`docs/09-open-decisions.md`](./docs/09-open-decisions.md): decisiones que faltan por confirmar.
- [`docs/references.md`](./docs/references.md): documentación oficial consultada.

## 9. Estado del proyecto

El desarrollo avanza feature por feature, con la autenticación al final (orden sugerido en [`docs/09-open-decisions.md`](./docs/09-open-decisions.md)).

| Feature | Estado |
|---|---|
| Catálogo (CRUD de SKU de PVC y manteles, con imagen de referencia) | Implementada (sin autenticación; solo entorno local) |
| Proveedores (CRUD de datos de contacto, activar y desactivar) | Implementada (sin autenticación; solo entorno local) |
| Compras (rollos físicos con código, stock de manteles, movimientos de inventario) | Implementada (sin autenticación; solo entorno local) |
| Inventario (existencias, rollos, movimientos y stock mínimo) | Implementado (solo consulta) |
| Ventas manuales (metros de rollo, rollo completo, manteles; solo registro: sin clientes, descuentos ni pagos) | Implementada (sin autenticación; solo entorno local) |
| Cobros, devoluciones, gastos, reportes, IA | Pendientes |
| Autenticación | Pendiente (última) |

Todavía no hay contrato OpenAPI final. Mientras no exista autenticación, el sistema solo debe ejecutarse en local y no exponerse públicamente.

## 10. Inicio de desarrollo (cuando se apruebe el alcance)

La estructura de repositorio sugerida es:

```text
project-root/
├── web/                 # Aplicación Vite + React + TypeScript (primera entrega)
├── backend/             # API Go con arquitectura hexagonal
├── android/             # Aplicación Kotlin / Jetpack Compose (etapa posterior)
├── docs/                # Documentación del producto y arquitectura
├── CLAUDE.md
└── README.md
```

### Ejecución local

Requisitos: Docker (el entorno usa OrbStack), Go y Node con pnpm.

1. Copia `.env.example` a `.env` y reemplaza los marcadores `<...>` por valores propios. `.env` no se versiona.
2. `make up` — levanta PostgreSQL y el simulador local de R2 (SeaweedFS).
3. `make migrate` — aplica las migraciones.
4. `make api` — inicia la API Go en `http://127.0.0.1:8080`.
5. `make web` — inicia el panel web en `http://localhost:5173`.

**Todo en contenedores (opcional).** `make app-up` construye y levanta los cuatro servicios: PostgreSQL, el almacenamiento, la API Go (aplica las migraciones al arrancar) y nginx (que sirve la web y reenvía `/api` a la API). La aplicación queda en `http://127.0.0.1:8081` (`WEB_PORT`). Por defecto solo es accesible desde la misma máquina, porque todavía no hay autenticación; con `WEB_BIND=0.0.0.0` en `.env` se abre a la red local (por ejemplo, para escanear QR con un celular). `make app-logs` sigue los registros y `make down` detiene todo sin borrar los datos. Todo lo de Docker está en la carpeta `docker/` (`docker-compose.yml`, `backend/` y `nginx/`); `make up` levanta solo PostgreSQL y el almacenamiento para trabajar con `make api` y `make web`. Sin `make`: `docker compose --env-file .env -f docker/docker-compose.yml up -d --build`.

Pruebas: `make test` (backend, sin contenedores), `make test-integration` (backend con PostgreSQL y almacenamiento reales) y `make web-test` (web). `make down` detiene los contenedores sin borrar los datos.

El despliegue en un servidor (HTTPS, dominio) y el proyecto Android no tienen todavía comandos; se añadirán cuando existan.
