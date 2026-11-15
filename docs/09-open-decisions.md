# 09. Decisiones pendientes

Este documento evita que el agente o el equipo invente reglas todavía no aprobadas. Mover cada punto a “Confirmado” solo después de una decisión explícita.

| Tema | Estado | Pregunta / decisión necesaria |
|---|---|---|
| Nombre definitivo | Pendiente | ¿Qué nombre tendrá la aplicación/proyecto? |
| Repositorios | Pendiente | ¿Web, Go y Android estarán en un monorepo (`web/`, `backend/`, `android/`) o repositorios separados? |
| Autenticación | Pendiente | ¿Habrá un solo usuario, varios usuarios y roles? ¿Qué mecanismo de autenticación? Se implementará como la última feature; hasta entonces el sistema corre solo en entorno local y no se expone públicamente. |
| Docker y despliegue de la BD | Pendiente | PostgreSQL en Docker está acordado; en desarrollo se usa la imagen `postgres:17` (propuesta, confirmar para producción). Backend y web ya se pueden ejecutar en contenedores (ver «Aplicaciones en contenedores» abajo). Falta decidir: dónde corre en producción, la política de respaldos del volumen y cómo se obtiene HTTPS. |
| Aplicaciones en contenedores | Propuesta (aplicada) | El usuario pidió dockerizar las aplicaciones con nginx en una carpeta de compose. Propuesta: todo en `docker/` con cuatro servicios en `docker/docker-compose.yml` —`postgres`, `object-storage`, `api` (imagen `docker/backend/Dockerfile`, Alpine, usuario sin privilegios; aplica las migraciones al arrancar) y `nginx` (`docker/nginx/`, sirve la web y reenvía `/api`)—; un solo origen, sin CORS; publicado en `127.0.0.1:8081` por defecto (`WEB_BIND`/`WEB_PORT`) porque no hay autenticación; las URLs firmadas de imágenes usan `OBJECT_STORAGE_PUBLIC_ENDPOINT`. Las migraciones corren al iniciar el contenedor de la API (válido con una sola instancia). Imágenes base: `golang:1.26.4-alpine`, `alpine:3.22`, `node:24-slim`, `nginxinc/nginx-unprivileged:1.28-alpine`. Para el VPS con Traefik hay un archivo adicional (`docker/docker-compose.traefik.yml`) con enrutamiento por subdominio, que no exige cambios en la web; un path (`/gumunova`) exigiría una base configurable en la web. Pendiente: dominio definitivo, elegir subdominio o path, si las imágenes seguirán en SeaweedFS (requiere un host público propio) o pasarán a Cloudflare R2, y proteger el acceso mientras no haya autenticación (sin protección, cualquiera que llegue al dominio puede usar el sistema). |
| Landing page | En espera | Página pública, sin fecha. Al reactivarla definir: contenido y objetivo, si muestra el catálogo (y cómo se expone sin autenticación), y si vive en el mismo proyecto Vite que el panel admin o en uno aparte. |
| Reglas del catálogo (implementadas) | Propuesta | SKU de 3 a 40 caracteres (`A-Z`, `0-9`, `-`, `_`; se guarda en mayúsculas); SKU y tipo inmutables; precios de referencia obligatorios y > 0; imagen JPG/PNG/WebP de hasta 5 MB, validada por contenido; URLs firmadas de 15 minutos. Confirmar o ajustar. |
| Reglas de proveedores (implementadas) | Propuesta | Solo el nombre es obligatorio; el correo se valida como dirección simple; no se exige nombre único; se desactiva en vez de borrar. Confirmar o ajustar. |
| Etiquetas con código QR / de barras | Propuesta (aplicada) | El usuario pidió generar e imprimir códigos QR o de barras de todos los productos o de los seleccionados. Acordado con el usuario: cada producto y cada rollo físico tiene un **código de etiqueta propio, único y regenerable** (separado del SKU y del código `ROL-…`); regenerarlo invalida las etiquetas ya impresas. Propuesta: formato `P-` (producto) o `R-` (rollo) + 12 caracteres hexadecimales aleatorios; QR o Code 128 a elección; etiqueta con código, SKU/código ROL y nombre; 1 a 50 copias; «todos» = productos activos o rollos con metros; se imprime con el diálogo del navegador. Librerías: `qrcode` y `jsbarcode`. Dirección del QR: el QR lleva `<dirección pública>/producto/<código>`; la dirección pública es `VITE_PUBLIC_BASE_URL` (ver `web/.env.example`) o, si está vacía, la dirección desde la que se abre la app al imprimir. Un QR impreso desde `localhost` no se puede abrir desde un celular (la pantalla de etiquetas lo advierte). Pendiente: elegir y publicar la dirección definitiva (https) y decidir si la consulta de precios debe pedir autenticación cuando esta exista. Escaneo (solo consulta): pantalla «Escanear» con cámara (API `BarcodeDetector` si el navegador la tiene; si no, `jsqr` solo para QR), foto del código o código tecleado/lector; muestra nombre, tipo y precios de referencia. La cámara en vivo exige https o localhost. Pendiente: tamaño de etiqueta/hoja de la impresora, si deben llevar precio, y usar el escaneo para agregar líneas a una venta. |
| Librerías adoptadas en el catálogo | Propuesta | Backend: `pgx` (PostgreSQL), `golang-migrate` (migraciones), AWS SDK for Go v2 (cliente S3/R2). Web: `react-router`, `vitest` y Testing Library. Se eligieron por ser estándar y mínimas; confirmar. |
| Despliegue de la web y CORS | Pendiente | En desarrollo Vite reenvía `/api` a la API, sin CORS. En el stack con contenedores nginx sirve la web y la API bajo el mismo origen (sin CORS). Falta decidir la topología de producción (mismo dominio o dominios distintos). |
| Código de rollo | Propuesta (aplicada) | Correlativo global `ROL-000001`, `ROL-000002`… generado por una secuencia de PostgreSQL (puede haber saltos si una compra falla). Etiqueta QR: pendiente. |
| Redondeo de importes | Propuesta (aplicada) | Los totales de línea (precio × cantidad) y los costos repartidos se redondean una sola vez a la centésima, hacia arriba en la mitad. Confirmar. |
| Costeo de ventas de PVC | Propuesta (aplicada) | El costo de lo vendido de un rollo se reparte en proporción a los metros, por diferencia de costo acumulado: las ventas de un rollo suman exactamente su costo. Manteles: promedio ponderado. Confirmar. |
| Descuentos, clientes y pagos en ventas | Pendiente | La primera versión de ventas solo registra la venta (precio efectivo por línea, sin descuento, cliente ni pago). Falta definir descuentos (línea o total, monto o porcentaje, tope), clientes (ficha o anónimo) y cobros (contado, parcial, medios de pago). |
| Anulación de ventas | Pendiente | Las ventas confirmadas no se editan; la anulación con movimientos compensatorios se definirá en la feature de devoluciones y ajustes. |
| Stock mínimo | Propuesta (aplicada) | Mínimo por variante: metros totales en PVC y unidades enteras en manteles, con aviso de «stock bajo» cuando la existencia es igual o menor. Cero = sin aviso. Falta decidir si habrá alertas de «rollos con pocos metros» por rollo individual. Confirmar. |
| Edición de compras | Propuesta (aplicada) | El usuario pidió poder editar y eligió «editar todo si nada se vendió». Detalles propuestos por confirmar: datos sin stock siempre editables; ítems reemplazables solo si nada de la compra se vendió (para manteles, ninguna venta del producto posterior a la compra); el reemplazo usa movimientos `purchase_reversal` y los rollos nuevos reciben códigos nuevos. La anulación completa sigue pendiente. |
| Orden de features | Propuesta | Desarrollo feature por feature, con autenticación al final. Orden sugerido: catálogo, compras/rollos, inventario, ventas manuales, cobros, devoluciones/ajustes, gastos, reportes, venta asistida por IA, autenticación. Confirmar. |
| Sucursales/depósitos | Pendiente | ¿Un solo inventario o varias ubicaciones? |
| Moneda | Propuesta | Bs/BOB como moneda inicial; confirmar si habrá otra moneda. |
| Longitud de rollo | Propuesta | Registrar longitud real utilizable en cada recepción, además del largo nominal. |
| Gastos atribuibles | Propuesta (aplicada) | Los costos adicionales se reparten entre todos los ítems de la compra en proporción a su costo, con el método del mayor resto (suma exacta). Se prefirió al reparto por metros porque funciona igual en compras que mezclan rollos y manteles; en rollos del mismo precio por metro equivale al reparto por metros. Confirmar. |
| Precios | Propuesta | Precio de referencia en SKU y precio final editable por venta; confirmar reglas/descuentos. |
| Costeo de manteles | Propuesta (aplicada) | Promedio ponderado: por SKU se guardan unidades y valor de existencias, y el costo promedio es valor ÷ unidades. Confirmar antes de implementar las ventas. |
| Rollo cortado | Propuesta | Después de cualquier salida parcial, el remanente solo se vende por metro. Confirmar. |
| QR/código de barras | Opcional | ¿Se usará QR impreso/etiqueta en cada rollo desde la primera versión? |
| Pagos parciales | Pendiente | ¿Se permitirán ventas a crédito, anticipos y pagos en varias cuotas? |
| Devoluciones | Pendiente | Plazo, condiciones, devolución parcial y criterios de reingreso a inventario. |
| Offline | Pendiente | Primera versión online; decidir si se necesita caché o registro offline. |
| Dictado | Propuesta | Usar IME/teclado Android; no grabar audio propio en la primera versión. |
| IA | Pendiente | Seleccionar modelo DeepSeek y definir límites/costo de uso, prompt y esquema JSON. |
| Exportación | Propuesta | Excel y PDF previstos; confirmar columnas y formato. |
| Documentos fiscales | Fuera de alcance inicial | Definir más adelante si se requiere factura o integración tributaria. |
| Eliminación y auditoría | Propuesta | Anular/revertir en vez de eliminar ventas/movimientos confirmados. |
| Android | Propuesta | Queda para una etapa posterior (no descartado). Confirmar si se mantiene como segunda fase. |
| Stack web | Pendiente | Vite + React + TypeScript está acordado; faltan router, fetch/cache de datos, librería de componentes/estilos, catálogo de textos en español y herramienta de pruebas. |
| Sesión web y CORS | Pendiente | Cómo se autentica la web y dónde se guarda la sesión/token; configuración CORS del backend. Depende de la decisión de autenticación. |
| Dictado en web | Pendiente | ¿Solo entrada escrita, dictado del sistema operativo o Web Speech API? |
| Alcance web inicial | Pendiente | ¿Qué módulos entran en la primera versión web (p. ej. catálogo, compras, ventas, inventario, reportes)? |

## Decisiones ya acordadas

- Primera aplicación: web, sencilla, con Vite + React + TypeScript.
- Proveedor de una compra: opcional. Sin proveedor se muestra «Sin proveedor» y los reportes por proveedor la agrupan aparte.
- Aplicación Android (etapa posterior): Kotlin y Jetpack Compose.
- Arquitectura Android (etapa posterior): Clean Architecture + MVVM + UDF.
- Backend: Go con arquitectura hexagonal.
- API: REST.
- Base de datos principal: PostgreSQL.
- Almacenamiento de archivos: Cloudflare R2.
- IA: DeepSeek invocada desde backend.
- Código/identificadores/contratos técnicos en inglés.
- Interfaz visible al usuario en español.
- Desarrollo feature por feature; la autenticación se trabaja al final.
- La web tiene dos áreas: panel admin (dashboard), que es el foco actual, y landing page, en espera.
- PostgreSQL se ejecuta en un contenedor Docker. El entorno local usa OrbStack como motor de Docker.
- En desarrollo local, Cloudflare R2 se simula con SeaweedFS (código abierto, API compatible con S3) en un contenedor; el backend usa el mismo adaptador S3 contra el simulador y contra R2, cambiando solo la configuración.
- Precisión de metros, anchos y longitudes: centésimas (0,01 m), dos decimales (`NUMERIC(12,2)`).
- El SKU se asigna manualmente y es único.
- El catálogo se desactiva (`is_active`) en vez de borrarse físicamente.
- El catálogo incluye imagen de referencia desde su primera versión (almacenada en el bucket S3/R2).
- Dos líneas de inventario separadas: PVC en rollos y manteles terminados por unidad.
- SKU para la variante de producto y código físico único para cada rollo.
- Ventas por UI, texto y dictado (teclado Android en esa app; en la web, pendiente), siempre con validación y confirmación.
- Módulo de reportes.
