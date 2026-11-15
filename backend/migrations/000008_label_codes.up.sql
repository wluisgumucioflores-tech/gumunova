-- Scannable codes for printed labels (QR or barcode). Each product and each physical roll has its
-- own unique code, separate from the SKU and the roll code, so it can be regenerated when a label
-- must stop working (lost, copied, or reprinted) without touching anything else.

-- Prefix + 12 hex characters (48 random bits): short enough for a small QR, long enough that a
-- clash is practically impossible; the unique constraints below catch it anyway.
CREATE FUNCTION new_label_code(prefix text) RETURNS text AS $$
    SELECT prefix || '-' || upper(substr(replace(gen_random_uuid()::text, '-', ''), 1, 12))
$$ LANGUAGE sql VOLATILE;

ALTER TABLE products ADD COLUMN label_code text;
UPDATE products SET label_code = new_label_code('P');
ALTER TABLE products
    ALTER COLUMN label_code SET NOT NULL,
    ALTER COLUMN label_code SET DEFAULT new_label_code('P'),
    ADD CONSTRAINT products_label_code_key UNIQUE (label_code);

ALTER TABLE pvc_rolls ADD COLUMN label_code text;
UPDATE pvc_rolls SET label_code = new_label_code('R');
ALTER TABLE pvc_rolls
    ALTER COLUMN label_code SET NOT NULL,
    ALTER COLUMN label_code SET DEFAULT new_label_code('R'),
    ADD CONSTRAINT pvc_rolls_label_code_key UNIQUE (label_code);
