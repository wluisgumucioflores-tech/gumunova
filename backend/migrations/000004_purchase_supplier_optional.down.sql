-- Rolling back requires every purchase to have a supplier again.
ALTER TABLE purchases ALTER COLUMN supplier_id SET NOT NULL;
