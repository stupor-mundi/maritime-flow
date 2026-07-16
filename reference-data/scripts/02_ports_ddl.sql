SET search_path = reference, public;

CREATE TABLE IF NOT EXISTS ports (
    id                  SERIAL PRIMARY KEY,
    port_name           TEXT NOT NULL,
    country             TEXT,
    port_type           TEXT,      -- commercial, military, etc.
    shelter             TEXT,      -- excellent, good, fair, poor, none
    entry_restriction   TEXT,
    max_vessel_size     TEXT,      -- very large, large, medium, small
    latitude            DOUBLE PRECISION NOT NULL,
    longitude           DOUBLE PRECISION NOT NULL,
    properties          JSONB,
    geom                geometry(Point, 4326)
);

CREATE INDEX IF NOT EXISTS ports_geom_idx
    ON ports USING GIST (geom);
CREATE INDEX IF NOT EXISTS ports_name_idx
    ON ports (port_name);
CREATE INDEX IF NOT EXISTS ports_country_idx
    ON ports (country);
CREATE INDEX IF NOT EXISTS ports_type_idx
    ON ports (port_type);
