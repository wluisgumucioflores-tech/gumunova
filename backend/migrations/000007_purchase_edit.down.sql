-- Fails if any purchase was edited: superseded history cannot be folded back into the old shape.
ALTER TABLE pvc_rolls DROP COLUMN superseded_at;
DROP INDEX purchase_lines_active_number_key;
ALTER TABLE purchase_lines DROP COLUMN superseded_at;
ALTER TABLE purchase_lines ADD CONSTRAINT purchase_lines_number_key UNIQUE (purchase_id, line_number);
ALTER TABLE purchases DROP COLUMN updated_at;
ALTER TABLE purchases DROP COLUMN revision;
ALTER TABLE inventory_movements DROP CONSTRAINT inventory_movements_movement_type_check;
ALTER TABLE inventory_movements
    ADD CONSTRAINT inventory_movements_movement_type_check CHECK (movement_type IN ('purchase_in', 'sale_out'));
