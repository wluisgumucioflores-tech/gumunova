-- Minimum stock per variant, used to flag low inventory. Zero means "no alert".
-- PVC is measured in meters across all of its rolls; tablecloths in whole units.
ALTER TABLE pvc_product_details
    ADD COLUMN minimum_stock_m numeric(12,2) NOT NULL DEFAULT 0 CHECK (minimum_stock_m >= 0);
ALTER TABLE tablecloth_product_details
    ADD COLUMN minimum_units integer NOT NULL DEFAULT 0 CHECK (minimum_units >= 0);
