# Referencias oficiales

Referencias consultadas para orientar la arquitectura y las integraciones. Revisar las guías antes de fijar versiones concretas de dependencias o contratos, ya que pueden cambiar.

## Android y Kotlin

1. **Android Developers — Recommendations for Android architecture**  
   https://developer.android.com/topic/architecture/recommendations  
   Capas UI/Data/Domain, repositorios, UDF, ViewModel, StateFlow, Coroutines y prácticas de pruebas.

2. **Android Developers — Guide to app architecture**  
   https://developer.android.com/topic/architecture  
   Separación de capas, repositorios, UI, dominio y principios de arquitectura moderna.

3. **Android Developers — State and Jetpack Compose**  
   https://developer.android.com/develop/ui/compose/state  
   Integración de Flows y `collectAsStateWithLifecycle()`.

4. **Android Developers — Dependency injection with Hilt**  
   https://developer.android.com/training/dependency-injection/hilt-android  
   Configuración de inyección de dependencias para Android.

5. **Android Developers — Localize your app**  
   https://developer.android.com/guide/topics/resources/localization  
   Recursos de texto, `strings.xml` y localización sin hardcodear textos.

6. **Android Developers — Jetpack Compose resources**  
   https://developer.android.com/develop/ui/compose/resources  
   Acceso a recursos de texto y `stringResource` desde Compose.

## Web

15. **Vite — Guide**  
   https://vite.dev/guide/  
   Herramienta de construcción y servidor de desarrollo de la aplicación web, incluido el manejo de variables `VITE_*` expuestas al cliente.

16. **React — Learn**  
   https://react.dev/learn  
   Componentes, estado y efectos de la interfaz web.

17. **TypeScript — Documentation**  
   https://www.typescriptlang.org/docs/  
   Tipado estático y modo estricto.

## Go

7. **The Go Programming Language — Organizing a Go module**  
   https://go.dev/doc/modules/layout  
   Guía de organización de módulos, paquetes, `internal/` y comandos del servidor.

## PostgreSQL

8. **PostgreSQL Documentation — Transactions**  
   https://www.postgresql.org/docs/current/tutorial-transactions.html  
   Operaciones atómicas con `BEGIN`, `COMMIT` y `ROLLBACK`; base para hacer consistentes ventas, movimientos y stock.

9. **PostgreSQL Documentation — Tutorial**  
   https://www.postgresql.org/docs/current/tutorial.html  
   Introducción a SQL, relaciones, claves foráneas y transacciones.

21. **pgx — PostgreSQL driver para Go**  
    https://github.com/jackc/pgx  
    Driver y pool de conexiones usados por el adaptador PostgreSQL.

22. **golang-migrate**  
    https://github.com/golang-migrate/migrate  
    Migraciones SQL versionadas, embebidas en el binario.

23. **AWS SDK for Go v2**  
    https://aws.github.io/aws-sdk-go-v2/docs/  
    Cliente S3 usado contra Cloudflare R2 y el simulador local; incluye URLs prefirmadas.

24. **React Router**  
    https://reactrouter.com/  
    Navegación del panel web.

25. **Vitest**  
    https://vitest.dev/  
    Pruebas unitarias y de componentes de la web.

## Docker

18. **Docker Hub — imagen oficial de PostgreSQL**  
    https://hub.docker.com/_/postgres  
    Variables de entorno, volúmenes de datos y versiones de la imagen oficial.

19. **Docker — Compose**  
    https://docs.docker.com/compose/  
    Definición y ejecución de varios contenedores para el entorno de desarrollo.

20. **SeaweedFS — Amazon S3 API**  
    https://github.com/seaweedfs/seaweedfs/wiki/Amazon-S3-API  
    Simulador local, de código abierto, del almacenamiento S3/R2 en desarrollo. Reemplaza a MinIO, cuyo repositorio comunitario fue archivado en 2026.

## Cloudflare R2

10. **Cloudflare — R2 overview**  
    https://developers.cloudflare.com/r2/  
    R2 como almacenamiento de objetos, no base de datos relacional.

11. **Cloudflare — How R2 works**  
    https://developers.cloudflare.com/r2/how-r2-works/  
    Arquitectura y consistencia de almacenamiento de objetos.

12. **Cloudflare — S3 compatibility**  
    https://developers.cloudflare.com/r2/api/s3/api/  
    Compatibilidad de la API S3 para integrar clientes y SDKs.

## DeepSeek

13. **DeepSeek API — Chat Completions**  
    https://api-docs.deepseek.com/api/create-chat-completion/  
    API de chat para enviar instrucciones y texto desde el backend.

14. **DeepSeek API — JSON Output**  
    https://api-docs.deepseek.com/guides/json_mode/  
    Respuesta JSON estructurada. Aunque se use JSON mode, el backend debe validar todos los campos y manejar respuestas vacías o truncadas.

## Etiquetas con código QR y de barras (web)

15. **node-qrcode** — https://github.com/soldair/node-qrcode  
    Genera la matriz del código QR; la web la dibuja como SVG (`QRCode.create`).

16. **JsBarcode** — https://github.com/lindell/JsBarcode  
    Genera códigos de barras Code 128 como SVG.

17. **jsQR** — https://github.com/cozmo/jsQR  
    Lee códigos QR desde píxeles; respaldo cuando el navegador no tiene `BarcodeDetector`.

18. **MDN — Barcode Detection API** — https://developer.mozilla.org/en-US/docs/Web/API/Barcode_Detection_API  
    Lectura nativa de QR y Code 128 en los navegadores que la incluyen.

19. **MDN — MediaDevices.getUserMedia()** — https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia  
    Acceso a la cámara; exige contexto seguro (https o localhost).

20. **Docker Compose — `depends_on` con `service_healthy`** — https://docs.docker.com/compose/how-tos/startup-order/  
    La API espera a PostgreSQL sano y nginx espera a la API sana.

21. **Dockerfile — compilaciones multi-etapa** — https://docs.docker.com/build/building/multi-stage/  
    Las imágenes finales no incluyen compiladores ni código fuente.

22. **nginx — imagen oficial sin privilegios** — https://hub.docker.com/r/nginxinc/nginx-unprivileged  
    Sirve la web y hace de proxy inverso hacia la API (`proxy_pass`: https://nginx.org/en/docs/http/ngx_http_proxy_module.html).

## Nota sobre estas referencias

- Las referencias técnicas justifican patrones y capacidades de las herramientas; no sustituyen las reglas de negocio de este proyecto.
- La arquitectura hexagonal se aplica como decisión de diseño del proyecto; Go no impone una estructura hexagonal oficial.
- No se fijaron versiones de Kotlin, AGP, Go, PostgreSQL, Compose, Hilt o clientes HTTP en este paquete. Se elegirán al crear los proyectos y se documentarán con sus versiones reales.
