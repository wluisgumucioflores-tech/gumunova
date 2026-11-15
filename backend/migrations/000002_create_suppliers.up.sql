-- Suppliers: businesses inventory is bought from. Deactivated, never deleted, so purchase
-- history keeps pointing at them.

CREATE TABLE suppliers (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL,
    phone      text        NOT NULL DEFAULT '',
    email      text        NOT NULL DEFAULT '',
    notes      text        NOT NULL DEFAULT '',
    is_active  boolean     NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT suppliers_name_not_blank CHECK (btrim(name) <> '')
);

CREATE INDEX suppliers_name_idx ON suppliers (name);
