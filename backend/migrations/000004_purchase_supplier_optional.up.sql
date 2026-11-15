-- A purchase may be registered without a supplier (for example, a cash purchase at a market).
ALTER TABLE purchases ALTER COLUMN supplier_id DROP NOT NULL;
