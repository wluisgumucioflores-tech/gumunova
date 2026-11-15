-- Catalog: commercial variants (SKU). Physical rolls are created later by purchases.
-- Money and measures use NUMERIC with two decimals (hundredths), never floating point.

CREATE TABLE products (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    sku          text        NOT NULL,
    name         text        NOT NULL,
    product_type text        NOT NULL,
    is_active    boolean     NOT NULL DEFAULT true,
    image_key    text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT products_sku_not_blank CHECK (btrim(sku) <> ''),
    CONSTRAINT products_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT products_type_valid CHECK (product_type IN ('pvc', 'tablecloth'))
);

CREATE UNIQUE INDEX products_sku_key ON products (sku);
CREATE INDEX products_type_idx ON products (product_type);

CREATE TABLE pvc_product_details (
    product_id                uuid          PRIMARY KEY REFERENCES products (id),
    material                  text          NOT NULL DEFAULT '',
    design                    text          NOT NULL DEFAULT '',
    color                     text          NOT NULL DEFAULT '',
    texture                   text          NOT NULL DEFAULT '',
    width_m                   numeric(12,2) NOT NULL CHECK (width_m > 0),
    nominal_length_m          numeric(12,2) NOT NULL CHECK (nominal_length_m > 0),
    thickness_mm              numeric(12,2) CHECK (thickness_mm > 0),
    reference_price_per_meter numeric(12,2) NOT NULL CHECK (reference_price_per_meter > 0),
    reference_price_per_roll  numeric(12,2) NOT NULL CHECK (reference_price_per_roll > 0)
);

CREATE TABLE tablecloth_product_details (
    product_id     uuid          PRIMARY KEY REFERENCES products (id),
    design         text          NOT NULL DEFAULT '',
    color          text          NOT NULL DEFAULT '',
    dimensions     text          NOT NULL DEFAULT '',
    reference_cost numeric(12,2) CHECK (reference_cost >= 0),
    unit_price     numeric(12,2) NOT NULL CHECK (unit_price > 0)
);
