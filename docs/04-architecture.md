# 04. Arquitectura del sistema

## 1. Visión general

```text
┌────────────────────────────────────┐
│ Android                            │
│ Kotlin + Jetpack Compose           │
│ Clean Architecture + MVVM + UDF    │
└─────────────────┬──────────────────┘
                  │ HTTPS / REST
                  ▼
┌────────────────────────────────────┐
│ Backend Go                         │
│ Hexagonal Architecture             │
│ HTTP adapter → use cases → domain  │
└─────────┬──────────────┬───────────┘
          │              │
          ▼              ▼
┌─────────────────┐  ┌─────────────────┐
│ PostgreSQL      │  │ Cloudflare R2   │
│ Datos de negocio│  │ Archivos/objetos│
└─────────────────┘  └─────────────────┘
          │
          └── Adaptador de IA ──► DeepSeek API
```

La primera aplicación cliente es la web (Vite + React + TypeScript); la aplicación Android llegará después. Ambas consumen la misma API Go y ninguna tiene una copia independiente de las reglas de inventario. En el diagrama, el recuadro "Android" representa a cualquiera de los clientes.

## 2. Backend Go — arquitectura hexagonal

### Domain

Contiene reglas e invariantes independientes de tecnología:

- Product/SKU, PVC roll, finished tablecloth.
- Purchase, sale, sale line, payment.
- Inventory movement, expense, supplier, customer.
- Value objects: money, quantity/length, identifiers.

El dominio no importa HTTP, SQL, SDKs de Cloudflare, ni clientes DeepSeek.

### Application

Contiene casos de uso y puertos:

- CreateProduct / UpdateProduct.
- RegisterPurchase.
- RegisterSaleDraftFromText.
- RegisterSale (confirmación humana).
- SellPvcByMeter / SellWholePvcRoll / SellTableclothUnits.
- RegisterDamage / RegisterReturn / ReverseSale.
- RegisterPayment / RegisterExpense.
- GetInventory / GetReports.

Los casos de uso orquestan el dominio, definen el límite transaccional y dependen de interfaces.

### Inbound adapters

HTTP handlers, middleware, autenticación, decodificación, validación de transporte y serialización. Los handlers no contienen lógica de inventario.

### Outbound adapters

- PostgreSQL: repositorios, transacciones, consultas y migraciones.
- DeepSeek: interpretación de textos y salida estructurada.
- Cloudflare R2: carga, descarga y URLs firmadas de archivos.

### Dirección de dependencias

```text
HTTP adapter ──► Application ──► Domain
Postgres adapter ──implementa puertos──► Application
DeepSeek adapter ──implementa puerto──► Application
R2 adapter ──implementa puerto──► Application
```

Los puertos deben definirse cerca de la capa que los necesita. Evitar una carpeta de interfaces genéricas sin contexto.

## 3. Web — Vite + React + TypeScript (primera entrega)

Aplicación de página única, sencilla, que consume la API Go.

- Tiene dos áreas: el **panel admin (dashboard)**, foco actual, y la **landing page**, en espera. Todo lo descrito en este documento y en `02`–`08` corresponde al panel admin.
- No se crea código ni estructura para la landing hasta que se reactive. Si más adelante comparte el proyecto Vite o es un proyecto aparte, está pendiente en `09-open-decisions.md`.
- La landing es pública y el panel admin es privado; esa frontera se hará efectiva con la autenticación (última feature).

- TypeScript en modo estricto; componentes funcionales de React.
- La UI no contiene reglas de negocio: muestra datos, envía acciones y presenta el resultado que el backend confirma.
- El cliente HTTP queda encapsulado en una capa de acceso a la API; los componentes no hacen `fetch` directamente.
- Los DTO de la API se mapean a tipos internos del frontend.
- Dinero y metros se reciben/envían como cadenas decimales; no se hacen cálculos monetarios con `number`.
- Todo lo que llega a `import.meta.env` / variables `VITE_*` queda expuesto en el bundle: nunca guardar secretos ahí.
- Como la web corre en el navegador, el backend deberá configurar CORS y definir cómo se almacena la sesión/token (pendiente, ver `09-open-decisions.md`).
- Librerías adicionales (router, fetch/cache, componentes UI, pruebas) no están decididas; se mantienen al mínimo y se documentan al adoptarlas.

## 3.1 Android — Clean Architecture + MVVM + UDF (etapa posterior)

### Presentation

- Jetpack Compose screens y componentes.
- ViewModel por pantalla o flujo relevante.
- `UiState` inmutable expuesto mediante StateFlow.
- Acciones de UI que se convierten en llamadas a use cases.
- Presentación de resultados, errores, carga, estados vacíos y confirmaciones.

### Domain

- Modelos de negocio utilizados por Android.
- Interfaces de repositorios.
- Use cases para orquestar operaciones utilizadas por varias pantallas o que tengan complejidad suficiente.
- Evitar duplicar reglas autoritativas de stock/costo que pertenecen al backend.

### Data

- Repositorios concretos.
- Fuentes remotas y locales.
- DTO de API, mappers y serialización.
- Gestión de errores de red y respuestas.

### Core

- Networking, navegación, design system, utilidades y configuración.

### Flujo de una venta

```text
SalesScreen
  └─ acción del usuario
     └─ SalesViewModel
        └─ RegisterSaleUseCase
           └─ SaleRepository (interface)
              └─ SaleRepositoryImpl
                 └─ Go REST API
```

Compose observa el `UiState` mediante colección lifecycle-aware (por ejemplo, `collectAsStateWithLifecycle`). La UI no accede directamente a la red o base de datos.

## 4. Estructura inicial sugerida

```text
project-root/
├── web/                      # Vite + React + TypeScript (primera entrega)
├── android/                  # etapa posterior
│   └── app/src/main/java/<package>/
│       ├── core/
│       │   ├── common/
│       │   ├── designsystem/
│       │   ├── navigation/
│       │   └── network/
│       ├── data/
│       │   ├── remote/
│       │   ├── local/
│       │   ├── mapper/
│       │   └── repository/
│       ├── domain/
│       │   ├── model/
│       │   ├── repository/
│       │   └── usecase/
│       └── presentation/
│           ├── dashboard/
│           ├── inventory/
│           ├── products/
│           ├── purchases/
│           ├── sales/
│           ├── suppliers/
│           ├── customers/
│           ├── reports/
│           └── settings/
├── backend/
│   ├── cmd/api/
│   ├── internal/domain/
│   ├── internal/application/
│   ├── internal/adapters/inbound/http/
│   ├── internal/adapters/outbound/postgres/
│   ├── internal/adapters/outbound/deepseek/
│   ├── internal/adapters/outbound/cloudflare_r2/
│   ├── migrations/
│   └── api/openapi/
├── docker/
│   ├── docker-compose.yml    # los cuatro servicios: nginx, api, postgres y object-storage
│   ├── backend/              # Dockerfile y entrypoint.sh de la API (migra y arranca)
│   └── nginx/                # Dockerfile (compila y sirve la web) y default.conf (proxy a /api)
├── docs/
├── CLAUDE.md
└── README.md
```

Es una propuesta para iniciar; no exige separar todo en módulos Gradle ni crear paquetes vacíos.

## 5. Persistencia

### PostgreSQL

La base de datos se ejecuta en un contenedor Docker.

- Los datos deben vivir en un volumen persistente, no dentro de la capa efímera del contenedor.
- Usuario, contraseña y nombre de la base se pasan por variables de entorno o un gestor de secretos; nunca se escriben en el repositorio.
- El backend Go se conecta por configuración (host/puerto/credenciales), sin asumir que la base es local.
- Las migraciones versionadas son la única forma de crear o cambiar el esquema.
- Cómo se despliega fuera del entorno local (Docker en un servidor, servicio gestionado, respaldos) está pendiente en `09-open-decisions.md`.

### Aplicaciones en contenedores

Todo lo de Docker vive en `docker/`. `docker/docker-compose.yml` define cuatro servicios —`nginx`, `api`, `postgres` y `object-storage`— (`make app-up`, o `docker compose --env-file .env -f docker/docker-compose.yml up -d --build` desde la raíz). Fija `name: gumunova` para que los volúmenes de datos conserven su nombre. `make up` levanta solo `postgres` y `object-storage`.

```text
navegador ──► nginx (puerto 8081 del host)
               ├─ /            → web estática (build de Vite) con respaldo a index.html para las rutas del cliente
               ├─ /api/        → api:8080 (API Go; no se publica fuera de la red de compose)
               └─ /healthz     → api:8080/healthz
api (aplica las migraciones al arrancar) → PostgreSQL, almacenamiento S3
```

- `docker/backend/Dockerfile` y `entrypoint.sh`: una imagen con los dos binarios (`/app/api` y `/app/migrate`), usuario sin privilegios. Al arrancar el contenedor se ejecuta `migrate` y, si termina bien, la API; si la migración falla, la API no inicia.
- `docker/nginx/Dockerfile` y `docker/nginx/default.conf`: compila la web y la sirve con `nginx-unprivileged`; los archivos con hash de `/assets` se cachean un año y `index.html` siempre se revalida. Como la web llama a `/api/v1` en el mismo origen, no hace falta CORS.
- Las credenciales no están en las imágenes ni en el repositorio: compose las toma de `.env`. `VITE_PUBLIC_BASE_URL` es un argumento de compilación público (dirección de los QR); vacío usa la dirección desde la que se abre la app.
- Las URLs firmadas de imágenes las abre el navegador, y la firma incluye el host. Por eso la API tiene `OBJECT_STORAGE_PUBLIC_ENDPOINT` (opcional): dirección con la que se firman las URLs, distinta de la interna `OBJECT_STORAGE_ENDPOINT` (`object-storage:8333`). En local apunta al puerto publicado del simulador; con Cloudflare R2 se deja vacía.
- No hay TLS en este stack: nginx escucha en HTTP. En un servidor con Traefik, `docker/docker-compose.traefik.yml` (`make app-up-traefik`) añade la red externa `traefik-proxy` y los labels (entrypoint `websecure`, resolver `letsencrypt`): Traefik termina el HTTPS y enruta `GUMUNOVA_HOST` a `nginx` y `GUMUNOVA_STORAGE_HOST` al almacenamiento (para las URLs firmadas de imágenes; con Cloudflare R2 no hace falta). Ver `09-open-decisions.md`.

Fuente de verdad para:

- Catálogo y SKU.
- Rollos físicos y metros disponibles.
- Inventario de manteles.
- Compras, ventas, líneas y pagos.
- Movimientos, devoluciones, anulaciones y gastos.
- Proveedores, clientes y auditoría.

Operaciones críticas deben ejecutarse en transacciones. Una venta no puede quedar registrada sin su correspondiente salida de stock ni actualizar stock sin una operación/movimiento trazable.

### Cloudflare R2

Almacén de objetos para:

- Imágenes de productos y diseños.
- Comprobantes y documentos adjuntos.
- Exportaciones temporales o archivos generados si se requiere.

En desarrollo local, R2 se reemplaza por un contenedor SeaweedFS con API S3 (ver `docker/docker-compose.yml`). El adaptador de almacenamiento habla S3, por lo que apunta al simulador o a R2 solo con configuración. Las imágenes del catálogo se limitan por tipo y tamaño y se sirven mediante URLs firmadas de corta duración; el bucket no es público.

Guardar en PostgreSQL solamente referencias/metadatos necesarios (object key, tipo, nombre, tamaño, etc.), no usar R2 como base de datos de negocio.

## 6. API

- REST sobre HTTPS.
- Contratos JSON con nombres de propiedades en inglés.
- Versionado recomendado bajo `/api/v1`.
- Códigos de error estables y validación en backend.
- OpenAPI como contrato documentado.
- Claves de idempotencia en operaciones de escritura.
- Paginación y filtros para listados/reportes.
- La aplicación web futura reutilizará la API.

Los endpoints concretos se definen en un contrato OpenAPI posterior; no se consideran cerrados en este documento.

### Catálogo (implementado)

Importes y medidas viajan como cadenas decimales con dos decimales (`"15.50"`). Los errores devuelven `{"error": {"code": "...", "fields": [{"field": "pvc.width_m", "code": "must_be_positive"}]}}`; Android y web traducen esos códigos al español.

| Método y ruta | Función |
|---|---|
| `POST /api/v1/products` | Crea un producto (`product_type`: `pvc` o `tablecloth`). |
| `GET /api/v1/products` | Lista con filtros `type`, `is_active`, `q` (SKU o nombre), `limit` (máx. 200) y `offset`. |
| `GET /api/v1/products/{id}` | Consulta un producto. |
| `PUT /api/v1/products/{id}` | Edita nombre y características. SKU y tipo no cambian. |
| `POST /api/v1/products/{id}/deactivate` / `activate` | Desactiva o reactiva; no se borra. |
| `POST /api/v1/products/{id}/label-code/reset` | Genera un nuevo `label_code` (código de etiqueta) del producto; el anterior deja de identificarlo. Editar el producto nunca cambia su código. |
| `PUT /api/v1/products/{id}/image` | Sube la imagen (`multipart/form-data`, campo `image`). |
| `DELETE /api/v1/products/{id}/image` | Quita la imagen. |
| `GET /healthz` | Comprobación de estado. |

Códigos de error de transporte y negocio: `validation_failed` (422), `invalid_json`, `invalid_query`, `invalid_upload` y `image_empty` (400), `product_not_found` (404), `sku_already_exists` (409), `image_too_large` (413), `image_type_not_supported` (415) e `internal_error` (500, sin detalles internos). La respuesta incluye `image_url`, una URL firmada de 15 minutos.

### Proveedores (implementado)

| Método y ruta | Función |
|---|---|
| `POST /api/v1/suppliers` | Crea un proveedor (`name` obligatorio; `phone`, `email` y `notes` opcionales). |
| `GET /api/v1/suppliers` | Lista con filtros `is_active`, `q` (nombre, teléfono o correo), `limit` (máx. 200) y `offset`. |
| `GET /api/v1/suppliers/{id}` | Consulta un proveedor. |
| `PUT /api/v1/suppliers/{id}` | Reemplaza los datos editables. |
| `POST /api/v1/suppliers/{id}/deactivate` / `activate` | Desactiva o reactiva; no se borra. |

Códigos adicionales: `supplier_not_found` (404) y, en errores de campo, `invalid_email`.

### Compras (implementado)

| Método y ruta | Función |
|---|---|
| `POST /api/v1/purchases` | Registra y confirma una compra. Exige el encabezado `Idempotency-Key`. Responde `201`, o `200` con `Idempotent-Replayed: true` si esa solicitud ya estaba registrada. |
| `GET /api/v1/purchases` | Lista (más recientes primero) con filtros `supplier_id`, `from`, `to` (fechas `AAAA-MM-DD`, inclusivas), `q` (comprobante o proveedor), `limit` (máx. 200) y `offset`. |
| `GET /api/v1/purchases/{id}` | Detalle con líneas, producto y los rollos creados con su código y costo atribuible, más `revision` e `items_editable`. |
| `PUT /api/v1/purchases/{id}` | Corrige una compra con el mismo cuerpo que el registro. Los datos siempre se actualizan; los ítems se reemplazan solo si cambian y nada de la compra se vendió (si no, `409 purchase_items_locked`). Ver `03-business-rules.md` §14. |

No hay rutas para editar ni eliminar una compra. Cuerpo de `POST`: `supplier_id` (opcional: si se omite o va vacío, la compra queda sin proveedor y las respuestas devuelven `"supplier": null`), `purchase_date`, `document_reference`, `notes`, `additional_costs` y `lines`; cada línea lleva `product_id` o, para un producto que aún no está en el catálogo, `new_product` (mismos campos que `POST /products`; se crea junto con la compra, en la misma transacción); una línea de PVC lleva `rolls` (`length_m` y `cost` de cada rollo) y una de mantel lleva `quantity` y `unit_cost`. Códigos adicionales: `purchase_not_found` (404), `idempotency_key_invalid` (400) e `idempotency_key_reused` (409); en errores de campo, `not_found`, `inactive`, `already_exists` (SKU de un producto nuevo ya tomado), `in_the_future`, `too_many` y `out_of_range`, con rutas como `lines[0].rolls[1].length_m`.

Ni el catálogo ni los proveedores usan clave de idempotencia: el SKU único impide duplicados de productos y un reintento de proveedor solo crea una ficha repetida, sin efecto sobre inventario ni dinero. Las operaciones de ventas y compras sí la requerirán.

## 7. Seguridad

- Credenciales y claves de proveedores solo en backend.
- Los clientes (web y Android) nunca contienen la clave DeepSeek ni credenciales directas PostgreSQL/R2.
- Autenticación/autorización obligatoria antes de producción; mecanismo pendiente de decisión.
- Acceso a archivos privados mediante URLs firmadas de vida limitada cuando corresponda.
- Validar entrada de IA como datos no confiables.

### Ventas (implementado)

| Método y ruta | Función |
|---|---|
| `GET /api/v1/labels/{code}` | Resuelve el código leído de una etiqueta: `kind` `product` o `roll`, con el producto (que trae los precios de referencia) y, si es rollo, su detalle. Ignora espacios y mayúsculas. Un código con formato inválido, desconocido o regenerado responde `404 label_not_found`. Un rollo reemplazado por la edición de una compra tampoco se resuelve. |
| `POST /api/v1/sales` | Registra y confirma una venta. Exige `Idempotency-Key`. Responde `201`, o `200` con `Idempotent-Replayed: true` si esa solicitud ya estaba registrada. |
| `GET /api/v1/sales` | Lista (más recientes primero) con filtros `from`, `to` (fechas `AAAA-MM-DD`, inclusivas), `q` (SKU o nombre de producto, o código de rollo), `limit` (máx. 200) y `offset`. |
| `GET /api/v1/sales/{id}` | Detalle con líneas, precio cobrado, precio de referencia, total, costo y ganancia de cada una. |

No hay rutas para editar ni eliminar una venta. Cuerpo de `POST`: `sale_date`, `notes` y `lines`. Cada línea lleva `kind` y `unit_price`: `roll_meters` (`roll_id`, `meters`), `full_roll` (`roll_id`) o `tablecloth_units` (`product_id`, `quantity`). Códigos adicionales: `sale_not_found` (404), `idempotency_key_invalid` (400) e `idempotency_key_reused` (409); en errores de campo, `insufficient_stock` (en `meters` o `quantity`), `roll_not_intact` (en `roll_id`), `not_found`, `inactive`, `wrong_type` e `in_the_future`, con rutas como `lines[1].meters`. La respuesta incluye `total_amount`, `total_cost` y `gross_profit`. Los movimientos de ventas aparecen en `GET /inventory/movements` con tipo `sale_out` y cantidad negativa.

### Inventario (implementado, solo lectura)

| Método y ruta | Función |
|---|---|
| `GET /api/v1/inventory/stock` | Existencias por variante: `quantity` (metros o unidades), `unit`, `minimum`, `is_low`, `roll_count` (solo PVC), `stock_value` y `average_cost` (solo manteles). Filtros `type`, `low=true`, `q`, `limit`, `offset`. |
| `GET /api/v1/inventory/rolls` | Rollos físicos con costo atribuible, `cost_per_meter`, `remaining_value` e `is_intact`. Filtros `product_id`, `status` (`available` por defecto, `depleted`, `all`), `q`, `limit`, `offset`. |
| `GET /api/v1/inventory/rolls/{id}` | Un rollo. Error `roll_not_found` (404). |
| `POST /api/v1/inventory/rolls/{id}/label-code/reset` | Genera un nuevo `label_code` del rollo. No cambia existencias ni escribe movimientos. Es la única escritura del inventario que no mueve stock. |
| `GET /api/v1/inventory/movements` | Registro de movimientos (más recientes primero). Filtros `product_id`, `roll_id`, `type`, `limit`, `offset`. |

Los parámetros inválidos responden `400 invalid_query`. `cost_per_meter`, `remaining_value`, `average_cost` y `stock_value` son valores de lectura redondeados; ningún cálculo los reutiliza. El catálogo incorpora `pvc.minimum_stock_m` y `tablecloth.minimum_units` (opcionales, por defecto cero).

## 8. Conectividad

La primera versión se plantea centrada en API online. La estrategia de uso sin conexión completa todavía no está decidida. Aun sin modo offline, la app debe mostrar errores de conectividad y evitar duplicar escrituras al reintentar.

## 9. Pruebas

- Backend: dominio y use cases unitarios; PostgreSQL y HTTP mediante integración.
- Web: pruebas de componentes y de la capa de API para los flujos críticos (herramienta pendiente de decidir).
- Android (etapa posterior): ViewModels/use cases/repositorios y pruebas UI de flujos críticos.
- Pruebas de concurrencia para impedir sobreventa.
