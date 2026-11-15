-- Manual sales. A confirmed sale is written in a single transaction, reduces stock and records
-- one outflow movement per line. It is never edited afterwards; mistakes are corrected with a
-- voiding that creates compensating movements (a later feature).
-- Money and measures are NUMERIC with two decimals, never floating point.

ALTER TABLE inventory_movements DROP CONSTRAINT inventory_movements_movement_type_check;
ALTER TABLE inventory_movements
    ADD CONSTRAINT inventory_movements_movement_type_check CHECK (movement_type IN ('purchase_in', 'sale_out'));

CREATE TABLE sales (
    id                  uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_date           date          NOT NULL,
    notes               text          NOT NULL DEFAULT '',
    -- Written once the lines are processed; the row is inserted first to act as the idempotency gate.
    total_amount        numeric(18,2) NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    total_cost          numeric(18,2) NOT NULL DEFAULT 0 CHECK (total_cost >= 0),
    status              text          NOT NULL DEFAULT 'confirmed' CHECK (status IN ('confirmed')),
    -- How the sale was entered, for audit: manual, text or dictation.
    entry_source        text          NOT NULL DEFAULT 'manual' CHECK (entry_source IN ('manual', 'text', 'dictation')),
    -- Makes a retry of the same request harmless: the same key never registers twice.
    idempotency_key     text          NOT NULL,
    request_fingerprint text          NOT NULL,
    created_at          timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT sales_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX sales_date_idx ON sales (sale_date DESC, created_at DESC);

CREATE TABLE sale_lines (
    id              uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id         uuid          NOT NULL REFERENCES sales (id),
    line_number     integer       NOT NULL CHECK (line_number > 0),
    -- roll_meters: meters cut from one roll; full_roll: an intact roll sold whole; tablecloth_units.
    sale_kind       text          NOT NULL CHECK (sale_kind IN ('roll_meters', 'full_roll', 'tablecloth_units')),
    product_id      uuid          NOT NULL REFERENCES products (id),
    roll_id         uuid          REFERENCES pvc_rolls (id),
    -- Meters (roll_meters), 1 (full_roll) or units (tablecloth_units).
    quantity        numeric(12,2) NOT NULL CHECK (quantity > 0),
    -- Snapshots: the price actually charged, and the catalog reference price at that moment.
    unit_price      numeric(12,2) NOT NULL CHECK (unit_price > 0),
    reference_price numeric(12,2) NOT NULL CHECK (reference_price > 0),
    line_total      numeric(18,2) NOT NULL CHECK (line_total > 0),
    cost_of_sale    numeric(18,2) NOT NULL CHECK (cost_of_sale >= 0),
    CONSTRAINT sale_lines_number_key UNIQUE (sale_id, line_number),
    CONSTRAINT sale_lines_roll_matches_kind CHECK ((sale_kind = 'tablecloth_units') = (roll_id IS NULL))
);

CREATE INDEX sale_lines_roll_idx ON sale_lines (roll_id);
CREATE INDEX sale_lines_product_idx ON sale_lines (product_id);
