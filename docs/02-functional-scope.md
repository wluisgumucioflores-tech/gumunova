# 02. Alcance funcional

## 1. Dashboard

- Mostrar ventas del período, ganancia bruta, inventario de PVC y unidades de manteles.
- Accesos rápidos a nueva venta y nueva compra.
- Alertas de stock mínimo, rollos con pocos metros y operaciones pendientes.
- No mostrar cifras ficticias; estados vacíos deben indicar que todavía no hay movimientos.

## 2. Catálogo de productos

### PVC

- Crear una variante/SKU por diseño y presentación comercial.
- Registrar nombre, material, modelo/diseño, color, textura, ancho, longitud nominal y espesor opcional.
- Guardar precio de referencia por metro y por rollo completo.
- Asociar imagen de catálogo opcional.

### Manteles terminados

- Crear SKU de producto terminado.
- Registrar diseño, color, dimensiones, unidad de medida, costo de referencia y precio de venta.
- Controlar existencias por unidad.

## 3. Compras

- Crear/seleccionar proveedor.
- Registrar fecha, referencia de factura/comprobante y observaciones.
- Agregar una o varias líneas.
- PVC: cantidad de rollos y datos reales por rollo (longitud, costo, gastos atribuibles). Cada rollo genera código único.
- Manteles: unidades compradas y costo unitario.
- Registrar gastos de compra atribuibles, como transporte, de manera que pueda definirse su asignación al costo de inventario.
- Confirmar compra y generar movimientos de entrada.

## 4. Inventario PVC

- Listado por SKU y listado de rollos físicos.
- Filtros por diseño, proveedor, estado y metros disponibles.
- Ficha individual del rollo: código, SKU, foto/diseño, largo inicial, saldo, costo, costo por metro, fecha/proveedor y movimientos.
- Registrar cortes, ventas, daños, devoluciones y ajustes justificados.
- Identificar rollos mediante búsqueda y, opcionalmente, QR/código de barras.

## 5. Inventario de manteles

- Stock total por SKU.
- Entrada por compra, salida por venta, devolución, daño y ajuste.
- Stock mínimo configurable y alerta de reposición.

## 6. Ventas

### Entradas admitidas

- UI: formulario.
- Texto escrito.
- Dictado: texto producido por el teclado/IME Android (en la web, pendiente de decisión).

### Líneas de venta

- PVC por metro: seleccionar rollo, cantidad decimal y precio por metro.
- PVC por rollo completo: seleccionar un rollo íntegro y precio por rollo.
- Mantel: seleccionar SKU, cantidad entera y precio unitario.

### Datos de la operación

- Fecha/hora, cliente opcional, líneas, descuentos, total, método de pago y estado de pago.
- Permitir registrar pago completo, parcial o saldo pendiente cuando se apruebe ese flujo.
- Mostrar vista previa de los datos, costo estimado y ganancia estimada antes de confirmar.
- Guardar snapshots de precio y costo usados al confirmar.

## 7. Cobros y cuentas pendientes

- Métodos previstos: efectivo, QR, transferencia y otros configurables.
- Registrar uno o más pagos asociados a una venta.
- Mostrar total, importe pagado y saldo pendiente.
- No considerar una venta pendiente como dinero cobrado.

## 8. Proveedores y clientes

- Datos básicos de contacto.
- Historial de compras por proveedor.
- Historial de ventas y saldos por cobrar por cliente.
- Cliente opcional para ventas de mostrador.

## 9. Movimientos y auditoría

Tipos iniciales: compra/entrada, venta/salida, corte, daño/desperdicio, devolución, ajuste de inventario y reversión/anulación.

Cada movimiento guarda fecha, referencia de operación, artículo/rollo afectado, cantidad, motivo y usuario/actor cuando exista autenticación.

## 10. Gastos

- Registrar importe, fecha, categoría, descripción y comprobante opcional.
- Ejemplos: transporte, embalaje y otros gastos operativos.
- Separar gastos operativos de costos de compra atribuibles al inventario.

## 11. Reportes

Ver `08-reporting.md` para indicadores y fórmulas. Filtros por período, línea de producto, SKU y modalidad. Exportación Excel/PDF prevista.

## 12. Fuera del alcance de la primera versión, salvo decisión posterior

- Landing page pública (en espera).
- Aplicación Android nativa (etapa posterior).
- Integración directa con facturación tributaria.
- Multiempresa, multialmacén o sucursales.
- Dictado de audio propio dentro de la aplicación; en primera versión se utiliza el teclado del sistema.
- Confirmación de ventas sin intervención humana mediante IA.
