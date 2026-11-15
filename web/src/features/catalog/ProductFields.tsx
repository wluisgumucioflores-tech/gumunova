import type { ReactNode } from 'react'
import type { ProductType } from '../../api/products'
import { BellIcon, CashIcon, InfoIcon, LockIcon, RulerIcon, SlidersIcon } from '../../components/icons'
import { TextField } from '../../components/TextField'
import { es } from '../../i18n/es'
import type { FieldKey, FormErrors, FormValues } from './formModel'

interface ProductFieldsProps {
  type: ProductType
  values: FormValues
  errors: FormErrors
  onChange: (key: FieldKey, value: string) => void
  /** Editing an existing product: the SKU is then read-only. */
  isEditing?: boolean
  /** Prefix of every input id, so the fields can appear more than once on a page. */
  idPrefix?: string
  /** Prefix of the API path of every field (data-path), used to focus the first invalid input. */
  pathPrefix?: string
  /** Shown first inside the general section, for example the product type selector. */
  lead?: ReactNode
}

/** The inputs of a catalog product. Used by the catalog form and by purchases that create a product. */
export function ProductFields({
  type,
  values,
  errors,
  onChange,
  isEditing = false,
  idPrefix = 'field-',
  pathPrefix,
  lead,
}: ProductFieldsProps) {
  const f = es.form.fields
  const hold = es.form.placeholders
  const decimal = { inputMode: 'decimal' as const }
  const money = { ...decimal, prefix: es.common.currencyPrefix, placeholder: hold.money }

  function field(key: FieldKey, label: string, options: Partial<Parameters<typeof TextField>[0]> = {}) {
    return (
      <TextField
        id={`${idPrefix}${key.replace('.', '-')}`}
        path={pathPrefix === undefined ? undefined : `${pathPrefix}${key}`}
        label={label}
        value={values[key]}
        error={errors[key]}
        onChange={(value) => onChange(key, value)}
        {...options}
      />
    )
  }

  return (
    <>
      <fieldset className="form-section">
        <legend>
          <InfoIcon />
          {es.form.sections.general}
        </legend>
        {lead}
        {field('sku', f.sku, {
          readOnly: isEditing,
          required: true,
          maxLength: 40,
          placeholder: hold.sku[type],
          hint: isEditing ? f.skuLocked : f.skuHint,
          hintIcon: <LockIcon />,
        })}
        {field('name', f.name, { required: true, maxLength: 120, placeholder: hold.name[type] })}
      </fieldset>

      {type === 'pvc' ? (
        <>
          <fieldset className="form-section">
            <legend>
              <SlidersIcon />
              {es.form.sections.pvc}
            </legend>
            <div className="form-grid">
              {field('pvc.material', f.material, { optional: true, maxLength: 80, placeholder: hold.material })}
              {field('pvc.design', f.design, { optional: true, maxLength: 80, placeholder: hold.design })}
              {field('pvc.color', f.color, { optional: true, maxLength: 80, placeholder: hold.color })}
              {field('pvc.texture', f.texture, { optional: true, maxLength: 80, placeholder: hold.texture })}
              {field('pvc.width_m', f.widthM, { ...decimal, required: true, suffix: es.form.units.meters })}
              {field('pvc.nominal_length_m', f.nominalLengthM, { ...decimal, required: true, suffix: es.form.units.meters, hint: f.nominalLengthHint })}
              {field('pvc.thickness_mm', f.thicknessMm, { ...decimal, optional: true, suffix: 'mm' })}
            </div>
          </fieldset>
          <fieldset className="form-section">
            <legend>
              <CashIcon />
              {es.form.sections.prices}
            </legend>
            <div className="form-grid">
              {field('pvc.reference_price_per_meter', f.pricePerMeter, { ...money, required: true })}
              {field('pvc.reference_price_per_roll', f.pricePerRoll, { ...money, required: true })}
            </div>
          </fieldset>
          <fieldset className="form-section">
            <legend>
              <BellIcon />
              {es.form.sections.stockAlert}
            </legend>
            <div className="form-grid">
              {field('pvc.minimum_stock_m', f.minimumStockM, {
                ...decimal,
                optional: true,
                suffix: es.form.units.meters,
                placeholder: hold.minimumStock,
                hint: f.minimumStockHint,
              })}
            </div>
          </fieldset>
        </>
      ) : (
        <>
          <fieldset className="form-section">
            <legend>
              <SlidersIcon />
              {es.form.sections.tablecloth}
            </legend>
            <div className="form-grid">
              {field('tablecloth.design', f.design, { optional: true, maxLength: 80, placeholder: hold.design })}
              {field('tablecloth.color', f.color, { optional: true, maxLength: 80, placeholder: hold.color })}
              {field('tablecloth.dimensions', f.dimensions, {
                optional: true,
                maxLength: 80,
                placeholder: hold.dimensions,
                hint: f.dimensionsHint,
                hintIcon: <RulerIcon />,
              })}
            </div>
          </fieldset>
          <fieldset className="form-section">
            <legend>
              <CashIcon />
              {es.form.sections.prices}
            </legend>
            <div className="form-grid">
              {field('tablecloth.reference_cost', f.referenceCost, { ...money, optional: true, hint: f.referenceCostHint })}
              {field('tablecloth.unit_price', f.unitPrice, { ...money, required: true, hint: f.unitPriceHint })}
            </div>
          </fieldset>
          <fieldset className="form-section">
            <legend>
              <BellIcon />
              {es.form.sections.stockAlert}
            </legend>
            <div className="form-grid">
              {field('tablecloth.minimum_units', f.minimumUnits, {
                ...decimal,
                optional: true,
                suffix: es.form.units.units,
                placeholder: hold.minimumStock,
                hint: f.minimumStockHint,
              })}
            </div>
          </fieldset>
        </>
      )}
    </>
  )
}
