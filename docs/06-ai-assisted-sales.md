# 06. Registro de ventas asistido por IA

## Objetivo

Facilitar el registro en español mediante texto escrito o dictado por el teclado Android, sin sacrificar el control humano ni la integridad del inventario.

## Modalidades

### Manual

El usuario selecciona producto, modalidad, rollo (si PVC), cantidad, precio, descuento y pago. El backend recalcula y valida al confirmar.

### Texto escrito

Ejemplo:

> Vendí 3 metros de PVC estampado floral a 15 bolivianos el metro.

### Dictado del teclado

- La pantalla presenta un campo de texto y un icono de micrófono.
- Al pulsar el icono, se enfoca el campo y se muestra el teclado del sistema.
- El usuario toca el micrófono de Gboard u otro IME y dicta.
- La app recibe el texto transcrito del teclado; no captura audio de forma propia en esta etapa.
- El micrófono y el idioma de dictado dependen del teclado instalado/configurado. Mantener siempre la alternativa de escribir.
- Esto describe el flujo Android (etapa posterior). En la web la entrada escrita es la base; si se ofrece dictado (por ejemplo, el del sistema operativo o la Web Speech API) es una decisión pendiente en `09-open-decisions.md`.

## Flujo técnico

```text
Android input (manual text / keyboard dictation)
        │
        ▼
POST Go API: parse sale instruction
        │
        ▼
Application use case
        │
        ▼
DeepSeek adapter (text prompt + JSON response mode)
        │
        ▼
Untrusted structured draft
        │
        ▼
Backend validation and product/roll matching
        │
        ▼
Android preview and user correction
        │
        ▼
Explicit confirmation
        │
        ▼
Go RegisterSale use case + PostgreSQL transaction
```

## Campos que puede proponer el parser

Un borrador podría incluir:

- `product_query` o candidato de `product_id`/`sku`.
- `sale_mode`: `pvc_by_meter`, `pvc_whole_roll`, `tablecloth_unit`.
- `quantity` y `unit`.
- `unit_price` y `currency` cuando el usuario lo indique.
- `roll_code` si se menciona explícitamente.
- `customer_name` opcional.
- `payment_method` y `paid_amount` si se mencionan.
- `missing_fields` y `ambiguities`.

La IA no debe inventar identificadores ni decidir silenciosamente entre varios productos/rollos similares.

## Validaciones obligatorias en Go

- La respuesta cumple el esquema esperado y no contiene datos inesperados que se utilicen.
- El SKU existe y corresponde al tipo de artículo.
- El modo de venta corresponde al producto.
- Para PVC por metro, existe un rollo seleccionado y tiene metros suficientes.
- Para rollo completo, el rollo está íntegro y no ha tenido cortes/salidas parciales.
- Para mantel, hay unidades suficientes.
- Cantidad y precio son válidos; precio/importe se recalculan en backend.
- Si hay ambigüedad o falta un campo esencial, devolver borrador incompleto y pedir edición.

## Confirmación y seguridad

- La IA nunca persiste una venta ni descuenta inventario.
- Mostrar una vista previa con producto, modalidad, rollo, cantidad, precio, total, costo/ganancia estimada y forma de pago.
- La acción “Confirmar venta” realiza una solicitud separada y explícita.
- La confirmación debe ser idempotente para evitar ventas duplicadas.
- API key de DeepSeek solo en backend.
- Registrar la fuente de captura (`manual`, `text`, `dictation`) si se necesita para auditoría.
- No almacenar grabaciones de audio; el alcance inicial solo procesa el texto transcrito.

## Contrato de IA

Usar salida estructurada (JSON mode si es compatible con el modelo seleccionado), pero no tratar JSON válido como dato verdadero. Validar siempre con el código de dominio. La documentación de DeepSeek advierte que la salida estructurada debe guiarse con instrucciones y que el cliente debe manejar contenido vacío o truncado.
