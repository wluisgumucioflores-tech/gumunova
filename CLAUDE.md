# CLAUDE.md — Reglas del proyecto

Este archivo contiene instrucciones obligatorias para cualquier agente que analice, genere o modifique código en este repositorio.

## 1. Antes de trabajar

1. Lee `README.md` y los documentos relevantes de `docs/` antes de implementar una funcionalidad.
2. `docs/03-business-rules.md` es la fuente de verdad para reglas de inventario, compras, ventas y cálculos.
3. `docs/04-architecture.md` es la fuente de verdad para límites arquitectónicos.
4. Si una solicitud contradice una regla documentada, identifica la contradicción y no cambies silenciosamente el comportamiento.
5. No inventes decisiones de negocio que estén marcadas como pendientes en `docs/09-open-decisions.md`. Propón una opción o solicita aclaración cuando sea imprescindible.
6. Actualiza la documentación afectada cuando una implementación cambie una regla, un flujo, un endpoint o una decisión arquitectónica.

## 2. Idioma y nomenclatura

- Todo el código se escribe en **inglés**: nombres de paquetes, módulos, clases, funciones, variables, interfaces, tipos, tablas, columnas, endpoints, propiedades JSON, constantes, tests y comentarios técnicos.
- Todo el texto que verá el usuario (web y Android) se escribe en **español**.
- No escribas textos visibles directamente dentro de funciones o composables. Decláralos como recursos Android (`strings.xml`) y accede mediante `stringResource(...)`.
- Usa nombres descriptivos, consistentes y específicos del dominio. Evita abreviaturas ambiguas.
- Los mensajes de API deben exponer códigos estables; Android los transforma a mensajes españoles. No presentar mensajes técnicos internos directamente al usuario.
- Documentación de producto: español. Documentación técnica puede ser español, pero los identificadores y ejemplos de código permanecen en inglés.

## 3. Reglas generales de diseño

- Prioriza claridad, separación de responsabilidades, testabilidad y consistencia sobre abstracciones innecesarias.
- Aplica SOLID cuando aporte valor práctico; no introduzcas capas, interfaces o patrones sin una razón concreta.
- No dupliques reglas de negocio entre Android y Go.
- PostgreSQL y el backend Go son la fuente de verdad de inventario y operaciones confirmadas.
- Nunca confíes en valores calculados o disponibilidad enviados por el cliente Android; vuelve a validarlos en el backend.
- No almacenes secretos en el repositorio. Usa variables de entorno/secret manager en backend y configuración segura para el entorno móvil.
- No registres tokens, contraseñas, claves API ni datos sensibles en logs.
- Maneja errores explícitamente; no ocultes excepciones ni devuelvas éxito ante una operación fallida.
- Usa tipos decimales adecuados para dinero y cantidades medibles. No utilices `float32`/`float64` ni `Double` para cálculos monetarios.
- Persiste fechas/horas con una representación consistente; presenta fechas al usuario en formato español y zona horaria configurada para el negocio.

## 4. Backend Go — Hexagonal Architecture

### Capas y dependencias

- **Domain:** entidades, value objects, invariantes y errores de dominio. No depende de HTTP, PostgreSQL, R2, DeepSeek ni frameworks.
- **Application:** casos de uso, coordinación, puertos y límites transaccionales. Depende del dominio.
- **Inbound adapters:** handlers HTTP, routing, validación de transporte y mapeo request/response. Invocan casos de uso; no contienen reglas de negocio.
- **Outbound adapters:** implementaciones de puertos para PostgreSQL, Cloudflare R2, DeepSeek y otros proveedores.
- La dirección de dependencias apunta hacia el dominio y los casos de uso; el dominio nunca importa adaptadores.

### Estructura sugerida

```text
backend/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── product/
│   │   ├── roll/
│   │   ├── inventory/
│   │   ├── purchase/
│   │   ├── sale/
│   │   ├── payment/
│   │   ├── supplier/
│   │   ├── customer/
│   │   ├── expense/
│   │   └── reporting/
│   ├── application/
│   │   ├── ports/
│   │   └── usecase/
│   └── adapters/
│       ├── inbound/http/
│       └── outbound/
│           ├── postgres/
│           ├── cloudflare_r2/
│           └── deepseek/
├── migrations/
├── api/
│   └── openapi/
└── go.mod
```

La estructura es una guía inicial, no una obligación de crear paquetes vacíos. Agrupa código por responsabilidad/dominio y evita paquetes genéricos enormes.

### Reglas de backend

- Los handlers HTTP traducen el transporte a comandos/consultas de aplicación; no calculan ganancias ni actualizan inventario directamente.
- Los casos de uso coordinan las operaciones y llaman a puertos definidos hacia dentro.
- Las implementaciones PostgreSQL quedan detrás de interfaces/puertos.
- Compras, ventas, pagos, devoluciones y movimientos relacionados deben usar una transacción de PostgreSQL cuando formen una sola operación de negocio.
- Bloquea o actualiza de forma segura el saldo del rollo/unidades para evitar vender existencias concurrentemente.
- Usa claves de idempotencia para operaciones de escritura susceptibles a reintentos o doble toque.
- Mantén un registro de movimientos auditable; no cambies existencias sin crear su movimiento correspondiente.
- Versiona migraciones; no modifiques migraciones ya aplicadas en entornos compartidos.
- PostgreSQL corre en un contenedor Docker con volumen persistente. Sus credenciales se pasan por variables de entorno y no se guardan en el repositorio (ni en `docker/docker-compose.yml` con valores reales).
- Valida todos los datos del request en el backend. La validación de Android es una ayuda de UX, no una frontera de seguridad.
- Define errores de dominio y tradúcelos a códigos HTTP/API consistentes en el adapter HTTP.
- Cubre los casos de uso y las reglas de dominio con pruebas unitarias; cubre transacciones/adaptadores con pruebas de integración.

## 5. Android Kotlin — Clean Architecture + MVVM + UDF

### Capas

- **Presentation:** pantallas Compose, componentes UI, ViewModels, `UiState` y acciones del usuario.
- **Domain:** modelos de dominio, contratos de repositorio y use cases. Debe ser Kotlin puro siempre que sea posible.
- **Data:** implementaciones de repositorios, DTO, mappers, clientes HTTP y fuentes locales/remotas.
- **Core:** componentes transversales como networking, navegación, design system, utilidades y configuración.

### Estructura sugerida

```text
android/app/src/main/java/<package>/
├── core/
│   ├── common/
│   ├── designsystem/
│   ├── navigation/
│   └── network/
├── data/
│   ├── remote/
│   ├── local/
│   ├── mapper/
│   └── repository/
├── domain/
│   ├── model/
│   ├── repository/
│   └── usecase/
└── presentation/
    ├── dashboard/
    ├── inventory/
    ├── products/
    ├── purchases/
    ├── sales/
    ├── suppliers/
    ├── customers/
    ├── reports/
    └── settings/
```

Empieza con módulos/paquetes simples y extrae módulos Gradle separados cuando exista una necesidad real. No conviertas cada clase en un módulo.

### Reglas Android

- Usa Jetpack Compose para la UI y una arquitectura de actividad única salvo necesidad justificada.
- Usa ViewModel a nivel de pantalla; expón el estado como `StateFlow<...UiState>` e inmutable hacia la UI.
- La UI emite acciones; el ViewModel coordina los casos de uso y actualiza el estado. Mantén flujo unidireccional de datos (UDF).
- Recoge Flows en Compose respetando el ciclo de vida, por ejemplo con `collectAsStateWithLifecycle()`.
- Los composables no deben llamar directamente a APIs, bases de datos ni repositorios.
- Los ViewModels no deben contener `Activity`, `Context` o `Resources` como dependencias ordinarias.
- Usa Coroutines/Flow para trabajo asíncrono; no bloquees el hilo principal.
- Usa Hilt para inyección de dependencias, salvo que una decisión documentada justifique otra opción.
- El cliente HTTP concreto debe quedar encapsulado en `data`; no acoples Domain a Retrofit/Ktor ni a DTO.
- Mapea DTO de API a modelos internos; no uses modelos de transporte directamente en toda la UI.
- Muestra estados de carga, vacío, error, éxito y confirmación cuando correspondan.
- Nunca consideres una venta confirmada hasta que el backend responda con éxito.
- Evita dobles envíos y trata correctamente timeouts/reintentos de escrituras mediante idempotencia en API.

## 5.1 Web — Vite + React + TypeScript (primera entrega)

La primera aplicación cliente es la web; Android queda para una etapa posterior. Mantén la web sencilla. Tiene dos áreas: el panel admin (dashboard), que es el foco actual, y la landing page, que está en espera: no crees código ni estructura para la landing hasta que el usuario la reactive.

- TypeScript en modo estricto y componentes funcionales de React.
- Los componentes no llaman directamente a `fetch`; el acceso a la API queda encapsulado en una capa propia y los DTO se mapean a tipos internos.
- La web no duplica reglas de negocio: valida solo como ayuda de UX y deja que Go decida.
- Dinero y metros viajan como cadenas decimales; no uses `number` para cálculos monetarios.
- Nunca pongas secretos en variables `VITE_*` ni en el código: todo lo que llega al bundle es público. La clave DeepSeek y las credenciales de PostgreSQL/R2 solo existen en el backend.
- Los textos visibles en español viven en un catálogo central, no dentro de los componentes.
- Muestra estados de carga, vacío, error, éxito y confirmación; una venta no está confirmada hasta que el backend responda con éxito.
- Evita dobles envíos (deshabilita el botón mientras la petición está en curso) y envía claves de idempotencia en escrituras.
- No añadas librerías (router, estado, UI, pruebas) sin una razón concreta; documenta la elección en `docs/references.md` y `docs/09-open-decisions.md`.

## 6. Inventario y operaciones: reglas no negociables

- SKU identifica una variante comercial, no un rollo físico.
- Cada rollo físico tiene un código único y mantiene su propio costo, longitud inicial, metros disponibles e historial.
- Se permiten fracciones de metro. No redondees internamente cantidades antes de calcular costos y saldos.
- El costo por metro se deriva del costo atribuible al rollo dividido entre su longitud utilizable inicial registrada.
- El precio de referencia por metro y el precio de referencia por rollo completo pertenecen a la variante/SKU. Un precio particular puede acordarse en una venta, pero debe quedar guardado como snapshot en la línea de venta.
- Una venta por metro debe referenciar el rollo físico exacto y descontar su longitud disponible.
- Una venta por rollo completo solo se permite si no tuvo cortes ni salidas parciales y se vende el rollo entero. Si fue cortado, el remanente se vende por metro.
- Las pérdidas por daño/desperdicio son movimientos de ajuste, no ventas.
- Una devolución debe inspeccionarse antes de reponer stock; registrar el resultado y el movimiento de devolución/ajuste.
- La venta de manteles descuenta unidades del SKU correspondiente.
- Mantén separados los inventarios y reportes de PVC y manteles, aunque ambos puedan aparecer en reportes consolidados.
- Una anulación revierte los efectos mediante movimientos compensatorios y conserva el registro original.
- No permitas stock negativo ni metros disponibles negativos.

## 7. IA y registro por texto/dictado

- La IA se invoca exclusivamente desde Go; nunca pongas la clave DeepSeek en Android.
- En la primera versión, el dictado utiliza el reconocimiento de voz del teclado/IME Android. La app trabaja con el texto producido; no necesita enviar audio.
- El icono de micrófono enfoca el campo de texto y presenta el teclado. No supongas que todos los IME tienen dictado ni que la app puede forzar su activación.
- DeepSeek debe devolver una estructura limitada a un esquema esperado (por ejemplo, JSON); trata todo resultado como entrada no confiable.
- Valida sintaxis, tipos, cantidades, referencias de SKU/rollo, precio, stock y reglas del dominio en Go.
- La IA puede crear un borrador, nunca confirmar ventas, asignar arbitrariamente un rollo ni modificar stock.
- Si falta información o hay ambigüedad, solicita corrección al usuario en la vista previa; no inventes campos.
- Registra origen del ingreso (`manual`, `text`, `dictation`) cuando aporte valor de auditoría, sin almacenar más texto personal del necesario.

## 8. Seguridad y configuración

- Android se comunica con Go por HTTPS.
- Las credenciales de PostgreSQL, Cloudflare R2 y DeepSeek solo existen en el backend/gestor de secretos.
- Aplica autenticación y autorización en backend antes de exponer operaciones de escritura. El mecanismo concreto queda pendiente de decisión.
- No confíes en identificadores, precios, permisos ni cantidades recibidas del cliente sin validación.
- No publiques buckets de R2 para escritura directa desde la app; si se habilitan cargas directas, usa URLs firmadas de corta duración generadas por Go y valida el acceso.
- Evita registrar contenido completo de ventas dictadas si no es necesario.

## 9. Calidad y pruebas

- Añade pruebas para toda regla de negocio nueva o modificada.
- Go: pruebas unitarias de dominio/casos de uso y pruebas de integración de PostgreSQL/HTTP.
- Web: pruebas de componentes y de la capa de API para los flujos críticos.
- Android: pruebas unitarias de use cases y ViewModels, pruebas de repositorios/mappers y pruebas UI/navigation para flujos críticos.
- Casos críticos mínimos: vender metros, vender rollo íntegro, impedir vender rollo cortado como completo, stock insuficiente, dos ventas concurrentes, devolución, anulación, daño, descuentos, pagos parciales y borrador IA ambiguo.
- Ejecuta formatter, análisis estático y pruebas disponibles antes de dar una tarea por terminada.
- No afirmes que las pruebas pasaron si no se ejecutaron.

## 10. Documentación y cambios

- Actualiza `README.md` si cambia el alcance, stack o estado general del proyecto.
- Actualiza los documentos de `docs/` si cambian reglas, arquitectura, modelo o flujos.
- Añade referencias oficiales en `docs/references.md` cuando se adopte o cambie una tecnología.
- Mantén `docs/09-open-decisions.md` actualizado; mueve una decisión a “Confirmada” solo cuando el usuario la haya aprobado.
- No inventes comandos de ejecución, variables de entorno, endpoints o versiones que no existan en el repositorio.
