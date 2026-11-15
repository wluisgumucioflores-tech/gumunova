-- Purchases, the physical PVC rolls they create, tablecloth stock and the movement ledger.
-- A confirmed purchase is written in a single transaction and never edited afterwards.
-- Money and measures are NUMERIC with two decimals, never floating point.

CREATE SEQUENCE roll_code_seq START 1;

-- Readable, unique code for each physical roll: ROL-000001, ROL-000002, ...
CREATE FUNCTION next_roll_code() RETURNS text AS $$
    SELECT 'ROL-' || CASE WHEN n < 1000000 THEN lpad(n::text, 6, '0') ELSE n::text END
    FROM (SELECT nextval('roll_code_seq') AS n) AS next_value
$$ LANGUAGE sql;

CREATE TABLE purchases (
    id                  uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id         uuid          NOT NULL REFERENCES suppliers (id),
    purchase_date       date          NOT NULL,
    document_reference  text          NOT NULL DEFAULT '',
    notes               text          NOT NULL DEFAULT '',
    additional_costs    numeric(18,2) NOT NULL DEFAULT 0 CHECK (additional_costs >= 0),
    total_amount        numeric(18,2) NOT NULL CHECK (total_amount > 0),
    status              text          NOT NULL DEFAULT 'confirmed' CHECK (status IN ('confirmed')),
    -- Makes a retry of the same request harmless: the same key never registers twice.
    idempotency_key     text          NOT NULL,
    request_fingerprint text          NOT NULL,
    created_at          timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT purchases_idempotency_key_key UNIQUE (idempotency_key)
);

CREATE INDEX purchases_date_idx ON purchases (purchase_date DESC, created_at DESC);
CREATE INDEX purchases_supplier_idx ON purchases (supplier_id);

CREATE TABLE purchase_lines (
    id              uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_id     uuid          NOT NULL REFERENCES purchases (id),
    line_number     integer       NOT NULL CHECK (line_number > 0),
    product_id      uuid          NOT NULL REFERENCES products (id),
    -- Tablecloth units, or the number of rolls for a PVC line.
    quantity        integer       NOT NULL CHECK (quantity > 0),
    -- Only tablecloth lines have a unit cost; PVC cost lives on each roll.
    unit_cost       numeric(12,2) CHECK (unit_cost > 0),
    additional_cost numeric(18,2) NOT NULL DEFAULT 0 CHECK (additional_cost >= 0),
    line_total      numeric(18,2) NOT NULL CHECK (line_total > 0),
    CONSTRAINT purchase_lines_number_key UNIQUE (purchase_id, line_number)
);

-- One row per physical roll. Each keeps its own cost, length and balance.
CREATE TABLE pvc_rolls (
    id                      uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    roll_code               text          NOT NULL,
    product_id              uuid          NOT NULL REFERENCES products (id),
    purchase_id             uuid          NOT NULL REFERENCES purchases (id),
    purchase_line_id        uuid          NOT NULL REFERENCES purchase_lines (id),
    line_position           integer       NOT NULL CHECK (line_position > 0),
    initial_usable_length_m numeric(12,2) NOT NULL CHECK (initial_usable_length_m > 0),
    -- Never negative and never above the initial length without a documented correction.
    available_length_m      numeric(12,2) NOT NULL,
    acquisition_cost        numeric(12,2) NOT NULL CHECK (acquisition_cost > 0),
    additional_cost         numeric(12,2) NOT NULL DEFAULT 0 CHECK (additional_cost >= 0),
    received_at             date          NOT NULL,
    created_at              timestamptz   NOT NULL DEFAULT now(),
    CONSTRAINT pvc_rolls_code_key UNIQUE (roll_code),
    CONSTRAINT pvc_rolls_available_valid CHECK (available_length_m >= 0 AND available_length_m <= initial_usable_length_m)
);

CREATE INDEX pvc_rolls_product_idx ON pvc_rolls (product_id);
CREATE INDEX pvc_rolls_purchase_idx ON pvc_rolls (purchase_id);

-- Tablecloth stock per SKU. The weighted average cost is stock_value / quantity, which keeps
-- the cost exact: it is derived, never rounded and stored.
CREATE TABLE tablecloth_stock (
    product_id  uuid          PRIMARY KEY REFERENCES products (id),
    quantity    integer       NOT NULL CHECK (quantity >= 0),
    stock_value numeric(18,2) NOT NULL CHECK (stock_value >= 0),
    updated_at  timestamptz   NOT NULL DEFAULT now()
);

-- Auditable ledger: stock never changes without a movement explaining why.
CREATE TABLE inventory_movements (
    id             uuid          PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id     uuid          NOT NULL REFERENCES products (id),
    roll_id        uuid          REFERENCES pvc_rolls (id),
    movement_type  text          NOT NULL CHECK (movement_type IN ('purchase_in')),
    quantity_delta numeric(18,2) NOT NULL CHECK (quantity_delta <> 0),
    unit           text          NOT NULL CHECK (unit IN ('m', 'unit')),
    reference_type text          NOT NULL,
    reference_id   uuid          NOT NULL,
    reason         text          NOT NULL DEFAULT '',
    created_at     timestamptz   NOT NULL DEFAULT now()
);

CREATE INDEX inventory_movements_product_idx ON inventory_movements (product_id, created_at);
CREATE INDEX inventory_movements_reference_idx ON inventory_movements (reference_type, reference_id);
