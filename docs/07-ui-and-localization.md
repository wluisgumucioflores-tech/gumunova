# 07. UI, experiencia de usuario e idioma

## Principios visuales

- Aplicación web (primera entrega) con Vite + React + TypeScript; interfaz sencilla y funcional, utilizable en escritorio y en pantallas pequeñas.
- Aplicación Android (etapa posterior) en Kotlin + Jetpack Compose + Material 3.
- Diseño claro, moderno y enfocado en operaciones rápidas.
- Formularios cortos y progresivos.
- Acciones principales visibles y fáciles de tocar.
- Mostrar unidades explícitas (m, rollo, unidad) y valores en Bs.
- Mostrar claramente carga, error, sin resultados, éxito y confirmación.
- Los valores de ganancia/costo son estimaciones hasta que el backend confirme la operación.

## Navegación principal

La web tiene un panel admin (dashboard) y una landing page en espera; esta navegación y las pantallas siguientes son las del panel admin. Cinco secciones (en Android, barra inferior; en la web, menú lateral o superior):

1. Inicio.
2. Inventario.
3. Ventas.
4. Compras.
5. Reportes.

Proveedores, clientes, gastos y ajustes pueden encontrarse desde sus módulos/contextos o desde secciones adicionales según evolucione el diseño.

## Pantallas prioritarias

### Inicio

- Tarjetas de ventas del período y ganancia bruta.
- Resumen: rollos, metros y manteles disponibles.
- Atajos a nueva venta y nueva compra.
- Alertas y actividad reciente.

### Inventario

- Selector o pestañas PVC / Manteles.
- PVC: vista por SKU y listado individual de rollos.
- Ficha del rollo con código, diseño, metros iniciales, metros disponibles, costo y movimientos.
- Manteles: stock por SKU/unidad.

### Nueva venta

- Selector de modalidad: Manual / Texto / Dictado.
- PVC: por metro o rollo completo.
- Mantel: por unidad.
- Elegir rollo cuando aplique.
- Mostrar cantidad, precio, descuento, costo estimado, total y ganancia estimada.
- Confirmación final con resumen comprensible.

### Nueva compra

- Seleccionar proveedor.
- Añadir rollos y manteles.
- Introducir datos reales y costos de cada rollo recibido.
- Revisar totales antes de confirmar.

### Reportes

- Rango de fechas.
- Filtros por línea, SKU, modalidad y proveedor.
- Indicadores y gráficos que no oculten los importes base.
- Exportación Excel/PDF.

## Idioma y localización

- Código fuente y contratos: inglés.
- Todo lo visible para el usuario: español.
- Crear recursos como `res/values/strings.xml` o `res/values-es/strings.xml` según la estrategia de locale seleccionada.
- Acceder a cadenas desde Compose con `stringResource(R.string...)`.
- Web: todos los textos visibles en español viven en un catálogo central de cadenas (mecanismo exacto pendiente); no se escriben directamente dentro de componentes.
- No hardcodear textos españoles/ingleses directamente en composables, ViewModels o mensajes de error.
- Formatear importes con locale español y moneda BOB/Bs; la configuración definitiva debe confirmarse.
- Las fechas se presentan en formato local del negocio; no construir fechas a partir de strings ambiguos.

## Icono de dictado

- El icono de micrófono de la app sirve para enfocar/abrir el campo de texto.
- El dictado lo presta el teclado Android (Gboard u otro IME), no un grabador propio de la app.
- No prometer que la app puede invocar de forma universal el micrófono interno del IME; su disponibilidad depende del teclado y la configuración del dispositivo.
- La entrada escrita debe estar siempre disponible.

## Web responsive (implementado)

- **Navegación:** hasta 860 px la barra lateral pasa a ser una barra superior fija; sus enlaces se desplazan en horizontal en lugar de apilarse.
- **Tablas:** en pantallas de hasta 700 px cada fila de las tablas de listado se muestra como una tarjeta, con cada valor rotulado con el nombre de su columna (`data-label`, tomado del encabezado). Entre 700 y 960 px la tabla se desplaza en horizontal dentro de su contenedor.
- **Formularios y filtros:** los filtros se apilan en una columna; las filas de rollos de una compra ponen el número y el botón de quitar en la primera línea y los campos a ancho completo; los botones de acción comparten la fila o ocupan todo el ancho.
- **Pantallas táctiles:** controles de al menos 44 px de alto (`pointer: coarse`).
- Al convertir una tabla en tarjetas los lectores de pantalla pierden la semántica de tabla en ese ancho; cada valor conserva su rótulo visible.


## Estilo visual del panel web (implementado)

Basado en el diseño de «Nuevo producto» entregado por el usuario:

- Tema claro y oscuro según el sistema; acento rojo de la marca. En ambos temas los campos de texto son claros para que se lean bien.
- Barra lateral con la tarjeta de marca (logo) y enlaces con icono; el enlace activo se resalta con el acento.
- Cada sección de un formulario es una tarjeta cuyo título es una etiqueta con icono sobre el borde superior.
- Campos: asterisco rojo en los obligatorios, etiqueta «opcional» en los opcionales, texto de ayuda con icono, moneda («Bs.») antes del valor y unidad (m, unidades) después, y ejemplos como texto de relleno.
- La barra de acciones del formulario (nota de campos obligatorios, Cancelar, Guardar) queda fija al pie mientras se desplaza.
- La imagen de referencia se elige en una zona punteada: clic o arrastrar un archivo.
- Pendiente de decisión (no implementado): «Generar SKU automático» (el SKU sigue siendo manual y único) e indicador «Sistema conectado» en la barra lateral.
