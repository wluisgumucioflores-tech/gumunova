DROP TABLE sale_lines;
DROP TABLE sales;
-- Outflow movements disappear with the sales that explain them.
DELETE FROM inventory_movements WHERE movement_type = 'sale_out';
ALTER TABLE inventory_movements DROP CONSTRAINT inventory_movements_movement_type_check;
ALTER TABLE inventory_movements
    ADD CONSTRAINT inventory_movements_movement_type_check CHECK (movement_type IN ('purchase_in'));
