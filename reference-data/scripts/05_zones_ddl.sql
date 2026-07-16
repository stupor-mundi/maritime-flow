SET search_path = reference, public;

CREATE TABLE IF NOT EXISTS coverage_zones (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    zone_type       TEXT NOT NULL
                    CHECK (zone_type IN (
                        'NORMAL',
                        'COVERAGE_HOLE',
                        'OCEAN',
                        'UNCLASSIFIED'
                    )),
    notes           TEXT,
    modified_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    geom            geometry(Polygon, 4326)
);

CREATE INDEX IF NOT EXISTS coverage_zones_geom_idx
    ON coverage_zones USING GIST (geom);
CREATE INDEX IF NOT EXISTS coverage_zones_type_idx
    ON coverage_zones (zone_type);

CREATE TABLE IF NOT EXISTS maritime_restrictions (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    restriction_type TEXT NOT NULL
                    CHECK (restriction_type IN (
                        'WAR_RISK',
                        'PIRACY',
                        'STRAIT',
                        'SANCTIONED_PORT',
                        'EEZ'
                    )),
    effective_from  DATE,
    effective_to    DATE,
    source          TEXT,
    notes           TEXT,
    modified_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    geom            geometry(Polygon, 4326)
);

CREATE INDEX IF NOT EXISTS maritime_restrictions_geom_idx
    ON maritime_restrictions USING GIST (geom);
CREATE INDEX IF NOT EXISTS maritime_restrictions_type_idx
    ON maritime_restrictions (restriction_type);

CREATE TABLE IF NOT EXISTS sensitive_locations (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    location_type   TEXT NOT NULL
                    CHECK (location_type IN (
                        'GREY_MARKET_HUB',
                        'STS_ZONE',
                        'SANCTIONED_TERMINAL',
                        'PORT_OF_INTEREST'
                    )),
    country         TEXT,
    notes           TEXT,
    source          TEXT,
    properties      JSONB,
    geom            geometry(Point, 4326)
);

CREATE INDEX IF NOT EXISTS sensitive_locations_geom_idx
    ON sensitive_locations USING GIST (geom);
CREATE INDEX IF NOT EXISTS sensitive_locations_type_idx
    ON sensitive_locations (location_type);
