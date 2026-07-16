SET search_path = reference, public;

CREATE TABLE IF NOT EXISTS anchorages (
    id                  SERIAL PRIMARY KEY,
    name                TEXT,
    anchorage_type      TEXT,      -- general, tanker, quarantine, etc.
    country             TEXT,
    properties          JSONB,
    geom                geometry(Polygon, 4326)
);

CREATE INDEX IF NOT EXISTS anchorages_geom_idx
    ON anchorages USING GIST (geom);
CREATE INDEX IF NOT EXISTS anchorages_type_idx
    ON anchorages (anchorage_type);
