import { listAllRolls } from '../../api/fetchAll'
import type { Roll } from '../../api/inventory'
import { es } from '../../i18n/es'
import { LabelsScreen, type LabelEntry } from '../labels/LabelsScreen'

const loadRolls = (everything: boolean, signal: AbortSignal) => listAllRolls(everything ? 'available' : 'all', signal)
const entryOf = (roll: Roll): LabelEntry => ({
  code: roll.label_code,
  primary: roll.code,
  secondary: `${roll.product.sku} — ${roll.product.name}`,
})

/** Labels of physical rolls: each carries the roll's own label code, so one roll is told apart from another of the same product. */
export function RollLabelsPage() {
  return <LabelsScreen<Roll> strings={es.inventory.rolls.labels} backTo="/inventario/rollos" loadItems={loadRolls} entryOf={entryOf} />
}
