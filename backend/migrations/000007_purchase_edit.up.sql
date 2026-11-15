-- Editing a confirmed purchase. Nothing is deleted: when the items of a purchase change, the
-- previous lines and rolls are marked superseded, a compensating movement cancels the stock
-- they added, and the new items are written as usual. The ledger keeps the whole history.

ALTER TABLE inventory_movements DROP CONSTRAINT inventory_movements_movement_type_check;
ALTER TABLE inventory_movements
    ADD CONSTRAINT inventory_movements_movement_type_check
    CHECK (movement_type IN ('purchase_in', 'purchase_reversal', 'sale_out'));

-- Number of times the items were replaced, plus one; and when the purchase was last edited.
ALTER TABLE purchases ADD COLUMN revision integer NOT NULL DEFAULT 1 CHECK (revision > 0);
ALTER TABLE purchases ADD COLUMN updated_at timestamptz;

ALTER TABLE purchase_lines ADD COLUMN superseded_at timestamptz;
ALTER TABLE purchase_lines DROP CONSTRAINT purchase_lines_number_key;
CREATE UNIQUE INDEX purchase_lines_active_number_key
    ON purchase_lines (purchase_id, line_number) WHERE superseded_at IS NULL;

ALTER TABLE pvc_rolls ADD COLUMN superseded_at timestamptz;
CREATE INDEX pvc_rolls_active_idx ON pvc_rolls (product_id) WHERE superseded_at IS NULL;
