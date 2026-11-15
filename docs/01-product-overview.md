# 01. Visión general del producto

## Propósito

Aplicación para controlar las compras, existencias, ventas y rentabilidad de un negocio que comercializa:

1. Rollos de PVC para mesas/manteles, vendidos por metro o rollo completo.
2. Manteles terminados, comprados a proveedores y revendidos por unidad.

El sistema debe responder con precisión: qué se compró, cuánto costó, qué queda, qué se vendió, qué cobro se recibió y cuál fue la rentabilidad.

## Alcance inicial

- Aplicación web sencilla con Vite + React + TypeScript (primera entrega), con dos áreas:
  - **Panel admin (dashboard):** operación interna del negocio (catálogo, compras, inventario, ventas, reportes). Es el foco actual.
  - **Landing page:** página pública. Está en espera (on hold); no se desarrolla hasta que se reactive.
- Backend HTTP en Go con arquitectura hexagonal.
- PostgreSQL como base de datos transaccional.
- Cloudflare R2 para archivos.
- DeepSeek para interpretar texto de venta, llamado desde backend.
- Idioma de interfaz: español.
- Identificadores y código: inglés.
- Aplicación Android nativa (Kotlin + Jetpack Compose): etapa posterior; compartirá la misma API.

## Principios de producto

- Trazabilidad individual por rollo físico.
- Separación del catálogo/SKU y las unidades físicas de inventario.
- Un único proceso de negocio para las tres formas de registrar ventas.
- Confirmación humana antes de operaciones sugeridas por IA.
- Historial auditable de todos los movimientos de existencias.
- Reportes que distingan ventas, costo de ventas, compras e inversión y gastos.

## Suposiciones iniciales

- El negocio se inicia con un único inventario/ubicación. Confirmar si existen sucursales o depósitos múltiples.
- Los importes de ejemplo se expresan en bolivianos (BOB/Bs); confirmar moneda y reglas de redondeo definitivas.
- Los rollos tienen ancho habitual de 1,40 m y largos nominales de 20 m o 30 m, pero debe poder registrarse la longitud real.
- Los manteles son productos terminados y se revenden por unidad.
