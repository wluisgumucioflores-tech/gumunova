ALTER TABLE pvc_rolls DROP COLUMN label_code;
ALTER TABLE products DROP COLUMN label_code;
DROP FUNCTION new_label_code(text);
