# 03. Reglas de negocio

Este documento es la referencia principal para implementar inventario, compras, ventas, costos y movimientos.

## 1. Catálogo, SKU y unidad física

- Un **SKU** identifica un producto o variante comercial (por ejemplo, un diseño de PVC de 20 m × 1,40 m).
- Un **rollo físico** es una unidad individual que se recibió en una compra y tiene su propio identificador (por ejemplo, `ROL-000125`).
- Varios rollos pueden compartir SKU, pero cada rollo tiene costo, longitud real y saldo independientes.
- El SKU no reemplaza al código del rollo. No crear un SKU distinto solo porque llegó otro rollo del mismo diseño y presentación.
- Manteles terminados se gestionan por SKU y cantidad de unidades; no generan registros individuales por pieza en la primera versión, salvo que se requiera trazabilidad unitaria.

## 2. Longitud del rollo

Cada rollo registra:

- `nominal_length_m`: largo anunciado/presentación nominal del SKU.
- `initial_usable_length_m`: longitud real inicial apta para venta, medida al ingresar.
- `available_length_m`: saldo actual disponible.
- `width_m`: ancho del material.

La longitud inicial puede ser diferente de la nominal. El saldo nunca puede ser negativo ni superar la longitud inicial sin un movimiento de corrección documentado.

## 3. Costos y precios de PVC

- El rollo guarda su costo de adquisición individual.
- Los gastos que se decida capitalizar (por ejemplo, flete directamente atribuible) se asignan al rollo mediante una regla documentada.
- `cost_per_meter = attributable_roll_cost / initial_usable_length_m`.
- No dividir por longitud nominal si se registró una longitud real utilizable diferente.
- La variante/SKU mantiene precios de referencia por metro y por rollo completo.
- Una venta puede tener un precio negociado distinto; guardar el precio efectivo como snapshot en la línea de venta.
- El costo de venta se calcula con el costo por metro del rollo específico, no con un costo promedio global del SKU.

## 4. Venta de PVC por metro

- Toda línea vendida por metro referencia un `roll_id`.
- Aceptar cantidades decimales (por ejemplo, 0,50 m; 1,25 m; 2,75 m).
- Antes de confirmar, comprobar que la cantidad sea mayor que cero y no supere el saldo disponible.
- Al confirmar: registrar la venta, crear el movimiento de salida y disminuir el saldo del rollo en una única transacción.
- Costo de venta de la línea = metros vendidos × costo por metro del rollo.
- Ganancia bruta de línea = importe neto de la línea − costo de venta.

## 5. Venta de rollo completo

- Solo se permite si el rollo está íntegro: no tuvo cortes, ventas parciales ni otras salidas que reduzcan su longitud.
- Se descuenta el rollo completo de inventario y se conserva su longitud/costo inicial en el historial.
- Su costo de venta es el costo atribuible total del rollo.
- Si el rollo ya fue cortado, su remanente se vende por metro, no como rollo completo.
- No permitir que dos operaciones vendan simultáneamente el mismo rollo completo.

## 6. Manteles terminados

- Compra: aumenta unidades disponibles por SKU.
- Venta: disminuye unidades disponibles por SKU.
- Validar stock antes de confirmar y bloquear ventas concurrentes que provoquen saldo negativo.
- Costo de venta = unidades vendidas × costo unitario asignado a esas unidades. La política de costeo (FIFO u otra) debe confirmarse antes de cerrar el modelo de datos.

## 7. Compras

- Una compra confirmada aumenta inventario y genera movimientos.
- Cada rollo de PVC recibido genera un registro físico y un costo individual.
- El registro de compra conserva proveedor, fecha, referencia documental, líneas, cantidades y gastos asociados.
- No actualizar compras históricas para cambiar costos ya consumidos sin un proceso explícito de ajuste.

## 8. Daños, desperdicio y ajustes

- Material dañado o desperdiciado no se registra como venta.
- Crear un movimiento de ajuste de salida con cantidad, fecha y motivo.
- Si se mide el costo de la pérdida, registrarlo por separado para análisis de merma.
- Los ajustes manuales requieren motivo obligatorio y deben quedar auditados.

## 9. Devoluciones y anulaciones

- No borrar físicamente una venta confirmada.
- La anulación/devolución crea movimientos compensatorios ligados a la operación original.
- El producto devuelto solo vuelve a estar disponible después de inspección y clasificación como apto para la venta.
- Material abierto/cortado o mantel dañado no debe reponerse automáticamente.
- Definir si se aceptan devoluciones parciales, cuánto tiempo y cómo se calcula el reembolso antes de habilitar la función completa.

## 10. Pagos

- Una venta y su pago son conceptos separados.
- Registrar el importe total de la venta, pagos recibidos y saldo por cobrar.
- Estados derivados sugeridos: `unpaid`, `partially_paid`, `paid`.
- No reportar saldo pendiente como ingreso cobrado.
- Los reembolsos deben quedar ligados a devolución/anulación y al pago original.

## 11. Integridad y concurrencia

- El backend es la autoridad de validación y cálculo.
- Confirmar venta, crear líneas, insertar movimientos, actualizar existencias y registrar pago aplicable en una transacción atómica.
- Aplicar bloqueo/actualización condicional al rollo o SKU para evitar sobreventa concurrente.
- Usar clave de idempotencia para evitar duplicados por doble toque, reintento de red o timeout.
- Restricciones de base de datos para cantidades positivas, relaciones y saldos válidos siempre que sea posible.
- Dinero en tipos decimales exactos (por ejemplo, `NUMERIC` en PostgreSQL). No usar punto flotante binario para importes.

## 12. Cálculos financieros

- Ventas netas = ventas confirmadas − descuentos − devoluciones correspondientes.
- Costo de ventas = costo asignado a las unidades/metros efectivamente vendidos.
- Ganancia bruta = ventas netas − costo de ventas.
- Resultado neto operativo = ganancia bruta − gastos operativos.
- Compras de inventario se presentan como inversión/entradas de inventario; no se descuentan completas de la ganancia del mes si parte del stock todavía no se vendió.
- Mostrar por separado ventas facturadas, cobros recibidos y saldos por cobrar.

## 13. IA

- Una interpretación de IA es un borrador, no una venta.
- El backend resuelve nombres/alias a SKU candidatos y valida el rollo elegido/disponible.
- Si la IA no identifica con claridad producto, modalidad, cantidad o precio, el borrador debe marcar esos campos como faltantes/ambiguos.
- La vista previa siempre permite editar y cancelar.
- Solo la acción explícita de confirmar crea la operación real.

## 14. Compras: reglas implementadas

Estas reglas desarrollan las secciones 2, 3, 6, 7 y 11 y están implementadas. Las decisiones marcadas como propuesta siguen en `09-open-decisions.md` hasta que se confirmen.

- **Confirmación inmediata.** Registrar una compra la confirma. Una compra confirmada no se elimina; se puede corregir según la regla «Edición» de abajo. La anulación completa (con movimientos compensatorios) sigue siendo una feature posterior.
- **Edición.** Proveedor, fecha, comprobante y notas se pueden corregir siempre, sin tocar el inventario. Las líneas, rollos, cantidades y costos (y los costos adicionales, que cambian el costo de cada rollo) solo se pueden reemplazar mientras **nada de lo que trajo la compra se haya vendido**: ningún rollo activo de la compra tiene ventas ni cortes y, para manteles, no hay ventas de ese producto posteriores al registro de la compra (las unidades de un SKU son intercambiables, así que cualquier venta posterior pudo usarlas). Si ya se vendió algo, el reemplazo se rechaza (`purchase_items_locked`, 409) y no cambia nada.
- **Cómo se reemplazan los ítems.** En una sola transacción: cada rollo anterior recibe un movimiento `purchase_reversal` negativo y queda marcado como reemplazado (queda en 0 y deja de aparecer en el inventario); las unidades de manteles anteriores se restan del stock con su `purchase_reversal`; las líneas anteriores se marcan como reemplazadas, y los ítems nuevos se escriben como en un registro (`purchase_in`, rollos con códigos nuevos). Nada se borra: el historial queda en el libro de movimientos y la compra incrementa su `revision`. Si los ítems no cambian, no se tocan los rollos ni sus códigos. Una edición idéntica repetida no cambia nada, por eso no exige clave de idempotencia.
- **Atomicidad.** La compra, sus líneas, los rollos, el aumento de stock y los movimientos se guardan en una única transacción: o se guarda todo o nada.
- **Idempotencia.** Registrar una compra exige el encabezado `Idempotency-Key`. Repetir la misma solicitud (doble toque, reintento tras un timeout) devuelve la compra ya registrada, sin duplicar rollos ni stock. Usar la misma clave con una compra distinta se rechaza.
- **Proveedor opcional.** Una compra puede registrarse sin proveedor (por ejemplo, una compra de contado a un particular). Si se indica, debe existir y estar activo. En listados y detalle se muestra como «Sin proveedor». Los costos, rollos y movimientos se tratan igual con o sin proveedor.
- **Producto nuevo desde la compra.** Una línea puede, en lugar de elegir un producto del catálogo, describir uno nuevo con los mismos datos y reglas que el catálogo (SKU único, tipo, precios de referencia…). El producto se crea activo y en la misma transacción que la compra: si la compra falla, no queda ningún producto huérfano, y un reintento con la misma clave de idempotencia no lo crea dos veces. Si el SKU ya existe, o se repite dentro de la misma compra, se rechaza en el campo `lines[n].new_product.sku`. La imagen se agrega después desde el catálogo.
- **Referencias válidas.** Cada producto elegido del catálogo debe existir y estar activo. El tipo de cada línea lo decide el catálogo, no el cliente: un producto PVC lleva rollos y un mantel lleva cantidad y costo por unidad.
- **Rollos.** Cada rollo recibido es un registro físico con código correlativo global (`ROL-000001`, `ROL-000002`…), su longitud real utilizable, su costo y un saldo inicial igual a su longitud. Se registran cantidades con centésimas de metro.
- **Costos adicionales (flete y similares).** Se reparten entre todos los ítems de la compra (cada rollo y cada línea de manteles) en proporción a su costo, con el método del mayor resto: las partes suman exactamente el total, sin perder ni inventar centavos. El costo atribuible de un rollo es su costo más su parte.
- **Costo por metro.** Se deriva como costo atribuible ÷ longitud inicial utilizable; no se redondea ni se guarda, para no perder precisión.
- **Manteles: promedio ponderado.** Por cada SKU se guardan unidades y valor de existencias; el costo promedio es valor ÷ unidades. Cada compra suma unidades y valor (costo más su parte de costos adicionales) en la misma transacción.
- **Movimientos.** Cada rollo genera un movimiento `purchase_in` en metros y cada línea de manteles uno en unidades, ligados a la compra. Las existencias nunca cambian sin su movimiento.
- **Fecha.** La fecha de compra no puede ser futura (se tolera un día por la zona horaria del negocio).
- **Límites de seguridad.** Hasta 100 líneas por compra, 100 rollos por línea y 100.000 unidades por línea de manteles.

## 15. Inventario: consultas implementadas

El inventario es de **solo lectura** en cuanto a existencias (la única escritura que no mueve stock es regenerar el código de etiqueta de un rollo, ver abajo): las existencias cambian únicamente mediante operaciones (hoy, compras) que escriben su movimiento en la misma transacción. No hay rutas para editar existencias a mano; los ajustes llegarán como operación propia con su movimiento.

- **Existencias por variante.** PVC se cuenta en metros disponibles sumando sus rollos, con el número de rollos que aún tienen metros; manteles se cuentan en unidades. Se mantienen separados y se pueden filtrar por tipo. Una variante inactiva solo aparece mientras conserve existencias.
- **Valor de lo que queda.** PVC: costo atribuible de cada rollo × metros disponibles ÷ longitud inicial (cada rollo con su propio costo, no un promedio del SKU). Manteles: valor de existencias guardado. Son valores de lectura redondeados a centésimas.
- **Costo promedio de manteles.** Valor de existencias ÷ unidades, con cuatro decimales solo para mostrar (el cálculo interno sigue exacto). En PVC no hay promedio: cada rollo tiene su costo por metro.
- **Rollos.** Se listan con longitud inicial, metros disponibles, costo atribuible, costo por metro (cuatro decimales, solo lectura) y valor restante. Por defecto solo los que tienen metros; también se pueden ver los agotados o todos. Un rollo es **íntegro** mientras su saldo iguale su longitud inicial (condición para poder venderlo completo).
- **Movimientos.** Historial ordenado del más reciente al más antiguo, filtrable por producto, rollo y tipo. Cada entrada indica cantidad con signo, unidad y la operación de origen.
- **Stock mínimo (propuesta aplicada).** Cada variante puede tener un mínimo: en metros totales para PVC (centésimas) y en unidades enteras para manteles. Hay aviso de **stock bajo** cuando el mínimo es mayor que cero y la existencia es igual o menor. Cero o vacío = sin aviso. Los productos con aviso aparecen primero.

- **Códigos de etiqueta.** Cada producto y cada rollo físico tiene un `label_code` único (`P-…` o `R-…` más 12 caracteres hexadecimales aleatorios), generado por la base de datos y distinto del SKU y del código `ROL-…`. Es lo que llevan los QR y códigos de barras impresos. Regenerarlo (acción explícita, con confirmación) lo cambia y las etiquetas impresas con el anterior dejan de identificarlo; no afecta existencias ni movimientos. Editar un producto no cambia su código. **Qué lleva cada etiqueta.** El QR lleva una dirección web (`<dirección pública>/producto/P-…`), así la cámara normal de cualquier celular la abre y muestra el producto y su precio; el código de barras lleva solo el código, para lectores. **Escanear.** La pantalla «Escanear» (menú) —también la que abre el QR— lee el código con la cámara, con una foto o tecleado (un lector USB/Bluetooth lo escribe solo) y muestra el nombre, el tipo y los precios de referencia: PVC por metro y por rollo completo; manteles por unidad. Si la etiqueta es de un rollo, muestra además ese rollo, sus metros disponibles y si puede venderse completo. Solo consulta: no registra ventas ni cambia nada. Un código regenerado o desconocido se informa como «no reconocido».

## 16. Ventas manuales: reglas implementadas

Desarrollan las secciones 4, 5 y 6. Esta primera versión **solo registra la venta**: no incluye clientes, descuentos, pagos ni cobros (siguen pendientes; ver `09-open-decisions.md`).

- **Tres tipos de línea.** `roll_meters`: metros de un rollo físico concreto (precio por metro). `full_roll`: un rollo íntegro completo (precio del rollo). `tablecloth_units`: unidades de un mantel (precio por unidad). Una venta puede mezclar los tres.
- **El producto lo decide el rollo o el catálogo**, nunca el cliente. Una línea de rollo no acepta `product_id`; una de mantel no acepta `roll_id`. El producto o rollo debe existir y estar activo.
- **Precio efectivo.** Cada línea guarda el precio realmente cobrado (puede diferir del de referencia) y, aparte, el precio de referencia del catálogo en ese momento (snapshot). Los precios deben ser mayores que cero.
- **Total de línea** = precio × cantidad (metros, 1 rollo o unidades), redondeado **una sola vez, a la centésima, hacia arriba en la mitad** (0,005 → 0,01), con aritmética entera.
- **Existencias.** No se permite vender más de lo disponible ni dejar saldos negativos. Si dos líneas de una misma venta toman del mismo rollo, la segunda ve lo que ya tomó la primera. Un rollo completo solo se vende si su saldo es igual a su longitud inicial (sin cortes ni ventas parciales); un rollo cortado se vende por metros. Se pueden vender todos los metros que quedan.
- **Costo de lo vendido (rollos).** Cada rollo reparte su costo atribuible en proporción a los metros vendidos. El costo de una venta es la diferencia entre el costo acumulado vendido después y antes de ella, de modo que los costos de todas las ventas de un rollo suman **exactamente** su costo, y la venta que lo agota toma lo que quede, sin perder ni inventar centavos. Un rollo completo cuesta su costo atribuible total.
- **Costo de lo vendido (manteles).** Promedio ponderado: valor de existencias × unidades vendidas ÷ unidades en existencia (redondeado a la centésima). Al venderse todas las unidades, el costo es exactamente el valor restante y no queda valor sin unidades.
- **Ganancia bruta** de línea y de venta = importe vendido − costo de lo vendido.
- **Atomicidad y concurrencia.** La venta, sus líneas, la reducción de existencias y un movimiento `sale_out` (negativo) por línea se guardan en una sola transacción. Los rollos y el stock de manteles se bloquean (`FOR UPDATE`) en orden fijo, así que dos ventas no pueden tomar los mismos metros ni el mismo rollo completo, y no hay interbloqueos.
- **Idempotencia.** Registrar exige el encabezado `Idempotency-Key`. Repetir la misma solicitud devuelve la venta ya registrada sin volver a descontar; la misma clave con otra venta se rechaza.
- **Confirmación.** Una venta no está confirmada hasta que el servidor responde con éxito. Una venta confirmada no se edita ni se elimina (la anulación con movimientos compensatorios es una feature posterior).
- **Fecha.** No puede ser futura (se tolera un día por la zona horaria). Hasta 100 líneas por venta, 100.000 unidades por línea de manteles. Cada venta guarda su origen (`manual` por ahora).

