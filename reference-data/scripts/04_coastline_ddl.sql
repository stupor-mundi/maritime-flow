SET search_path = reference, public;

CREATE TABLE IF NOT EXISTS coastline (
    id                  SERIAL PRIMARY KEY,
    name                TEXT,
    continent           TEXT,
    geom                geometry(MultiPolygon, 4326)
);

CREATE INDEX IF NOT EXISTS coastline_geom_idx
    ON coastline USING GIST (geom);
