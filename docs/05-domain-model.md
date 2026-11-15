# 05. Modelo de dominio conceptual

> Modelo conceptual inicial; no es todavía un esquema SQL definitivo. Los nombres se expresan en inglés para mantener la convención del código.

## Entidades principales

| Entidad | Responsabilidad | Campos conceptuales |
|---|---|---|
| `Product` | Producto/variante del catálogo | `id`, `sku`, `name`, `category`, `product_type`, `is_active` |
| `PvcProductDetails` | Atributos específicos de PVC | `product_id`, `material`, `design`, `color`, `texture`, `width_m`, `nominal_length_m`, `thickness_mm` |
| `TableclothDetails` | Atributos de mantel terminado | `product_id`, `design`, `color`, `dimensions`, `unit_of_measure` |
| `PvcRoll` | Rollo físico individual | `id`, `roll_code`, `product_id`, `purchase_line_id`, `initial_usable_length_m`, `available_length_m`, `attributable_cost`, `received_at` |
| `Purchase` | Cabecera de compra | `id`, `supplier_id` (opcional), `purchase_date`, `document_reference`, `status`, `total_amount` |
| `PurchaseLine` | Línea de compra | `id`, `purchase_id`, `product_id`, `quantity`, `unit_cost`, `additional_cost_allocation` |
| `Sale` | Cabecera de venta | `id`, `sale_number`, `customer_id`, `sold_at`, `status`, `subtotal`, `discount`, `total`, `entry_source` |
| `SaleLine` | Detalle vendido | `id`, `sale_id`, `product_id`, `roll_id`, `sale_mode`, `quantity`, `unit_price`, `unit_cost_snapshot`, `line_total` |
| `Payment` | Cobro asociado a venta | `id`, `sale_id`, `amount`, `method`, `paid_at`, `reference` |
| `InventoryMovement` | Libro de movimientos | `id`, `product_id`, `roll_id`, `movement_type`, `quantity_delta`, `unit`, `reference_type`, `reference_id`, `reason`, `created_at` |
| `Supplier` | Proveedor | `id`, `name`, `phone`, `email`, `notes` |
| `Customer` | Cliente opcional | `id`, `name`, `phone`, `notes` |
| `Expense` | Gasto operativo | `id`, `category`, `description`, `amount`, `expense_date` |
| `Attachment` | Archivo guardado en R2 | `id`, `object_key`, `content_type`, `file_name`, `size_bytes`, `related_type`, `related_id` |

### Implementado en la feature Catálogo

- `Product`: la implementación omite `category` (el campo `product_type` basta por ahora), y `TableclothDetails` omite `unit_of_measure` (los manteles se venden por unidad). Se añadirán si surge la necesidad.
- Tablas: `products`, `pvc_product_details`, `tablecloth_product_details`. Importes y medidas son `NUMERIC(12,2)`; el SKU es único; la imagen se guarda como `image_key` (clave del objeto en R2).
- SKU y tipo de producto son inmutables tras la creación. Un producto se desactiva (`is_active`), no se borra.
- Los precios de referencia del PVC (por metro y por rollo) y el precio unitario del mantel son obligatorios y mayores que cero. El costo de referencia del mantel y el espesor del PVC son opcionales.

### Implementado en la feature Proveedores

- Tabla `suppliers` con `name`, `phone`, `email`, `notes` e `is_active`. Solo el nombre es obligatorio (hasta 120 caracteres); teléfono hasta 30, correo hasta 254 con formato válido y notas hasta 500.
- Un proveedor se desactiva (`is_active`), no se borra, para conservar el historial de compras.
- No se exige nombre único: dos negocios pueden llamarse igual. Se puede revisar si resulta confuso.

### Implementado en la feature Compras

- Tablas: `purchases`, `purchase_lines`, `pvc_rolls`, `tablecloth_stock` e `inventory_movements`; la secuencia `roll_code_seq` y la función `next_roll_code()` generan los códigos de rollo.
- Códigos de etiqueta (migración 000008): `products.label_code` y `pvc_rolls.label_code` (únicos, con valor por defecto de la función `new_label_code(prefijo)`).
- Edición (migración 000007): `purchases.revision` y `updated_at`; `purchase_lines.superseded_at` y `pvc_rolls.superseded_at` marcan lo reemplazado (no se borra nada); el movimiento `purchase_reversal` cancela el stock de lo reemplazado. Las lecturas ignoran lo reemplazado.
- `purchases` guarda `idempotency_key` (único) y `request_fingerprint`, y su `status` es `confirmed`. Los totales son `NUMERIC(18,2)`; los costos por ítem, `NUMERIC(12,2)`.
- `pvc_rolls` guarda `roll_code` (único), `initial_usable_length_m`, `available_length_m`, `acquisition_cost` y `additional_cost`; restricciones: saldo no negativo y nunca mayor que la longitud inicial.
- `tablecloth_stock` guarda `quantity` y `stock_value` (no negativos); el costo promedio es `stock_value / quantity`.
- `inventory_movements` es el libro auditable (`purchase_in` por ahora; cada operación futura añade sus tipos).
- La tabla `Payment` y las ventas siguen pendientes.

## Relaciones principales

- Un `Product` tiene un SKU único por variante.
- Un `Product` PVC puede tener muchos `PvcRoll` físicos.
- Un `Purchase` tiene una o más `PurchaseLine`.
- Una compra de PVC genera uno o más rollos físicos, uno por cada unidad recibida.
- Un `Sale` tiene una o más `SaleLine` y puede tener uno o más `Payment`.
- Una `SaleLine` de PVC por metro o rollo referencia exactamente un `PvcRoll`.
- Un movimiento se vincula al producto y, cuando aplica, al rollo y a la operación que lo originó.
- Proveedores/clientes pueden estar asociados a compras/ventas.

## Identificadores

- `sku`: código comercial estable de una variante.
- `roll_code`: identificador legible único de cada rollo físico, por ejemplo `ROL-000125`.
- IDs internos: UUID u otro identificador técnico consistente. La elección definitiva se define al diseñar el esquema.
- QR/barcode puede codificar el identificador del rollo, pero no sustituye la validación del backend.

## Consideraciones para el esquema físico

- Usar tipos decimales exactos para dinero y longitud (por ejemplo, `NUMERIC` en PostgreSQL) con escala decidida por reglas de negocio.
- Añadir restricciones para cantidades positivas, claves foráneas y unicidad de SKU/código de rollo.
- Evitar guardar el saldo de stock sin su relación con el libro de movimientos; si se materializa para rendimiento, actualizarlo en la misma transacción.
- Guardar snapshots de precio y costo en las líneas de venta para preservar el cálculo histórico aunque cambie el precio/costo del catálogo.
- Definir política de costeo de manteles (FIFO, promedio ponderado u otra) antes del DDL definitivo.
