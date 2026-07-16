# reference-data

PostGIS DDL scripts and data loading scripts for all reference
layers used by maritime-flow. Reference data lives in a `reference`
schema in the PostGIS container on Minisforum.

---

## Schema overview

```
reference schema (PostGIS):

  ports                 global port locations and polygons
                        source: World Port Index (WPI), NGA
                        ~3,700 ports worldwide

  anchorages            offshore anchorage areas
                        source: OpenStreetMap
                        important for STS transfer detection

  coastline             land/sea boundary
                        source: Natural Earth (10m resolution)
                        used for position-on-land spoofing detection

  coverage_zones        AIS receiver coverage classification
                        derived empirically from accumulated data
                        not hand-drawn — see design note below
                        zone_type: NORMAL | COVERAGE_HOLE | OCEAN
                                   UNCLASSIFIED (default)

  maritime_restrictions war risk zones, piracy areas,
                        strait closures, EEZ boundaries
                        hand-drawn in QGIS, stored in PostGIS
                        type: WAR_RISK | PIRACY | STRAIT |
                              SANCTIONED_PORT | EEZ

  sensitive_locations   known grey-market ports, sanctioned
                        terminals, STS transfer zones
                        hand-drawn or from OSINT sources
```

---

## Data sources

| Layer | Source | Format | Licence |
|---|---|---|---|
| ports | World Port Index (NGA) | CSV | Public domain (US government) |
| anchorages | OpenStreetMap | GeoJSON/Shapefile via Overpass | ODbL |
| coastline | Natural Earth | Shapefile | Public domain |
| coverage_zones | Derived from accumulated NMEA data | PostGIS (native) | N/A |
| maritime_restrictions | Hand-drawn in QGIS | PostGIS (native) | N/A |
| sensitive_locations | OSINT + hand-drawn | PostGIS (native) | N/A |

---

## Scripts

```
reference-data/scripts/
├── 01_schema_ddl.sql           creates reference schema + extensions
├── 02_ports_ddl.sql            ports table + spatial index
├── 03_anchorages_ddl.sql       anchorages table + spatial index
├── 04_coastline_ddl.sql        coastline table + spatial index
├── 05_coverage_zones_ddl.sql   coverage_zones + maritime_restrictions
│                               + sensitive_locations tables
├── 06_load_ports.sql           loads WPI CSV via COPY
├── 07_load_anchorages.sql      loads OSM anchorage GeoJSON
├── 08_load_coastline.sql       loads Natural Earth shapefile
└── download/
    ├── download_wpi.sh         downloads WPI CSV from NGA
    ├── download_coastline.sh   downloads Natural Earth shapefile
    └── extract_anchorages.sh   Overpass API query for OSM anchorages
```

---

## Running the scripts

**Prerequisites:**
- PostGIS container running (see k8s/README.md or docker run)
- DBeaver, psql, or any PostgreSQL client
- CSV/shapefile data downloaded (see download/ scripts)

**Order matters — run in sequence:**

```bash
# Copy data files into PostGIS container first
docker cp data/wpi_ports.csv postgis:/tmp/wpi_ports.csv
docker cp data/ne_coastline.shp postgis:/tmp/ne_coastline.shp

# Run DDL scripts
psql -h localhost -U postgres -f scripts/01_schema_ddl.sql
psql -h localhost -U postgres -f scripts/02_ports_ddl.sql
psql -h localhost -U postgres -f scripts/03_anchorages_ddl.sql
psql -h localhost -U postgres -f scripts/04_coastline_ddl.sql
psql -h localhost -U postgres -f scripts/05_coverage_zones_ddl.sql

# Load data
psql -h localhost -U postgres -f scripts/06_load_ports.sql
psql -h localhost -U postgres -f scripts/07_load_anchorages.sql
psql -h localhost -U postgres -f scripts/08_load_coastline.sql
```

**DBeaver note:** Use `COPY` (server-side) not `\copy` (client-side).
Files must be accessible inside the Docker container — use `docker cp`
to copy them in before running load scripts.

All scripts use `SET search_path = reference, public;` — the
PostGIS extension functions (geometry types, ST_ functions) live
in `public`, reference data tables land in `reference`.

---

## Coverage zones — empirical derivation

Unlike the air-cargo project where coverage gaps are global and
relatively stable (China suppresses ADS-B, the Atlantic has no
receivers), maritime-flow operates a local receiver with a specific
166km range footprint. Coverage zones are better derived from the
data than drawn by hand.

**Approach:**

After accumulating weeks of NMEA data, grid the sea area and count
observed vessel positions per cell:

```sql
SELECT
    ROUND(latitude / 0.5) * 0.5   AS lat_bin,
    ROUND(longitude / 0.5) * 0.5  AS lon_bin,
    COUNT(*) AS position_count
FROM delta_scan('./delta/tracks')
GROUP BY lat_bin, lon_bin
```

High density cells → NORMAL coverage.
Zero density adjacent to high density → likely gap.
Zero density in open water beyond ~166km → your range limit, expected.

This self-corrects as data accumulates and automatically handles
complex coastlines (Greek islands, Turkish coast) that would be
tedious to draw by hand. All zones start as UNCLASSIFIED.

**Incremental improvement:**

Run analysis with UNCLASSIFIED zones → gaps appear in results.
Review obvious cases: open Mediterranean = OCEAN, busy coastal
lane with zero count = COVERAGE_HOLE. Draw those polygons.
Refine over time as flagged gaps reveal zone boundaries that need
adjusting. The same incremental feedback loop as air-cargo, just
starting from data rather than domain knowledge.

## Maritime restrictions — hand-drawn

`maritime_restrictions` and `sensitive_locations` are drawn
directly in QGIS against the PostGIS tables:

**Maritime restrictions** encode areas relevant to vessel routing
and anomaly context:

```
WAR_RISK         Red Sea (Houthi attacks), Black Sea
PIRACY           Gulf of Aden, Gulf of Guinea
STRAIT           Suez Canal, Bosphorus, Strait of Hormuz
                 chokepoints where routing is constrained
SANCTIONED_PORT  ports subject to UN sanctions
EEZ              Exclusive Economic Zones for fishing analysis
```

**Sensitive locations** — known grey-market hubs, STS transfer
zones, ports associated with sanctions evasion. Sources: OSINT,
published investigative journalism, UN panel reports.

Draw in QGIS → save to PostGIS → ais-analyser reads via JDBC.
No export/import step — QGIS edits in place.

**Airspace restrictions** (overflight bans, conflict zones) are
an air-cargo concept and do not belong in maritime-flow.
Do not add airspace restriction polygons to this module.

---

## PostGIS schema style

Hybrid approach: typed columns for core fields, JSONB `properties`
column for flexible attributes. GIN index on `properties` for
efficient JSONB queries.

Geometry columns use `geometry(Polygon, 4326)` or
`geometry(Point, 4326)` — always EPSG:4326 (WGS84). GIST index
on all geometry columns.

---

## Current status

```
01_schema_ddl.sql           ⬜ not yet written
02_ports_ddl.sql            ⬜ not yet written
03_anchorages_ddl.sql       ⬜ not yet written
04_coastline_ddl.sql        ⬜ not yet written
05_coverage_zones_ddl.sql   ⬜ not yet written
Load scripts                ⬜ not yet written
Download scripts            ⬜ not yet written
Coverage zones              ⬜ derived from data — needs
                               sufficient NMEA accumulation first
Maritime restrictions       ⬜ not yet drawn
Sensitive locations         ⬜ not yet drawn
```
