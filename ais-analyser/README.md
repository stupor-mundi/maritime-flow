# ais-analyser

Batch analytical engine for maritime intelligence. Reads accumulated
raw NMEA sentences from Redpanda on the Pi 4, decodes vessel
positions, applies spatial enrichment and anomaly scoring, and
writes results to Delta Lake.

Runs on demand on a local Minisforum workstation via K3s Spark
Operator. Not a continuous streaming job — data accumulates in
Redpanda while the Minisforum is off, and is processed in batch
when analysis is wanted.

---

## What it does

```
Pi 4 Redpanda: ais.nmea (raw NMEA, accumulated)
    → Spark reads batch of accumulated messages
    → NMEA decoding → AIS position records
    → spatial enrichment via SedonaSQL + PostGIS
        port proximity (World Port Index)
        anchorage proximity (OSM)
        position on land (coastline — spoofing signal)
        coverage zone classification
        airspace restriction zone check
    → anomaly scoring
    → Delta Lake writes:
        vessels    enriched per-vessel state (MERGE)
        tracks     position history (append)
        gaps       gap records when vessel goes dark (append)
```

---

## Why Spark not Flink

This is a batch analytical workload, not a streaming one. Data
accumulates in Redpanda on the always-on Pi 4. When analysis is
wanted, Spark processes the entire accumulated dataset in one batch.

Flink's streaming strengths — stateful per-key processing, continuous
output, event-time watermarks — are not needed here. SparkSQL and
SedonaSQL provide a clean, SQL-like interface for the spatial joins
and aggregations that make up most of the analytical work.

The companion project air-cargo uses Flink where streaming is
genuinely required (per-aircraft stateful flight phase detection).

---

## Technology choices

**Apache Spark + SedonaSQL**

SedonaSQL registers spatial functions directly in SparkSQL —
`ST_Within`, `ST_DWithin`, `ST_DistanceSphere`, `ST_MakeLine` and
others are available as SQL functions in Spark queries. This makes
spatial joins against PostGIS reference layers readable and concise:

```sql
SELECT p.mmsi, p.latitude, p.longitude, port.name
FROM positions p
JOIN ports port
ON ST_DWithin(
    ST_Point(p.longitude, p.latitude)::geography,
    port.geom::geography,
    5000  -- 5km threshold
)
```

**Delta Lake**

Spark is Delta Lake's primary engine. MERGE semantics, OPTIMIZE,
ZORDER, and time travel are all first-class in the Spark/Delta
combination. Vessel records are updated via MERGE rather than
rewritten — landing enrichment and anomaly score updates can
be applied to existing records cleanly.

**DuckDB for ad-hoc queries**

After each analysis run, Delta Lake tables can be queried directly
with DuckDB for ad-hoc exploration:

```sql
-- DuckDB
INSTALL delta; LOAD delta;
SELECT mmsi, anomaly_score, last_port
FROM delta_scan('./delta/vessels')
WHERE anomaly_score > 0.7
ORDER BY anomaly_score DESC;
```

---

## Delta Lake tables

```
vessels           one row per vessel (MMSI)
                  MERGE on update — anomaly scores, port history
                  partitioned by flag_state

tracks            one row per observed position (Tier 1 only)
                  append only — never update individual positions
                  partitioned by date + mmsi
                  OPTIMIZE ZORDER BY (latitude, longitude)

gaps              one row per detected gap
                  written when vessel reappears after silence
                  Tier 1 facts: last observed position,
                  first reobserved position, polls missed,
                  implied speed, gap classification
```

### Track point tiers

Three tiers of position data — must never be mixed:

```
Tier 1: OBSERVED      real AIS positions from RTL-SDR receiver
                      full analytical value, stored in tracks

Tier 2: INFERRED      analytically deduced positions
                      stored in gaps table with confidence score

Tier 3: TESSELLATION  geometric rendering nodes for great circle
                      display — NO analytical value
                      stored only in implied_tracks display table
                      never joined into analytical queries
```

---

## Gap classification

When a vessel goes dark and reappears, the gap is classified against
the coverage zone layer in PostGIS:

```
OCEAN             vessel disappeared over open water
                  landing impossible — voyage continued
                  draw implied great circle track

COVERAGE_HOLE     vessel disappeared in area with no receivers
                  (Central Asia, Central Africa, parts of Russia)
                  ambiguous — duration is the only signal

REGULATORY_SUPPRESSION  vessel disappeared in a data suppression zone
                        (China, North Korea)
                        treat as data void — absence expected

ANOMALOUS         vessel disappeared in a well-covered area
                  high anomaly signal — transponder off?
                  interception? technical failure?
```

---

## Anomaly scoring

Fuzzy 0.0-1.0 scores. No binary thresholds. Signals:

```
Dark period in ANOMALOUS coverage zone    high (0.7-0.9)
AIS position on land                      hard anomaly (0.95)
                                          — spoofing signal,
                                          do NOT filter out
Implied speed physically impossible       hard anomaly (0.95)
Ship-to-ship transfer (STS detection)     medium (0.5-0.7)
Port call in sanctioned location          high (0.7-0.9)
Unexpected flag state change              medium (0.4-0.6)
Gap in COVERAGE_HOLE zone                 low (0.1-0.2)
Gap in OCEAN zone                         near zero (0.0-0.1)
```

Scores accumulate in the vessels Delta Lake table. Alert when
score is sustained above threshold over a time window — not on
a single data point.

---

## Spatial operations

SedonaSQL functions used:

```sql
ST_DWithin(geom, point, metres)    port/anchorage proximity
ST_Within(point, polygon)          coverage zone, restricted zone
ST_DistanceSphere(p1, p2)          gap implied speed calculation
ST_MakeLine(p1, p2)                implied track geometry
ST_Intersects(line, polygon)       what does implied track cross?
ST_Point(longitude, latitude)      point construction
                                   note: longitude first
```

PostGIS reference layers accessed via JDBC from Spark:

```scala
val ports = spark.read
  .format("jdbc")
  .option("url", postgisUrl)
  .option("dbtable", "reference.ports")
  .load()
```

---

## Docker image

Do NOT use the `apache/sedona` image for K3s deployment — it bundles
its own Spark master/worker and clashes with the Spark Operator.

Build from `apache/spark` and layer Sedona on top:

```dockerfile
FROM apache/spark:3.4.1

ENV SEDONA_PKGS="org.apache.sedona:sedona-spark-shaded-3.4_2.12:1.6.0,\
org.datasyslab:geotools-wrapper:1.6.0-28.2"

RUN ${SPARK_HOME}/bin/spark-shell \
    --packages ${SEDONA_PKGS} \
    --repositories https://repo1.maven.org/maven2 \
    --conf spark.jars.ivy=/tmp/ivy_cache < /dev/null
```

See [k8s/README.md](../k8s/README.md) for Spark Operator deployment.

---

## Running locally (development)

Without K3s, run Spark in local mode for development and testing:

```bash
cd ais-analyser
mvn package
spark-submit \
  --master local[*] \
  --packages org.apache.sedona:sedona-spark-shaded-3.4_2.12:1.6.0 \
  target/ais-analyser-0.1.0.jar
```

---

## Configuration

```bash
KAFKA_BOOTSTRAP_SERVERS=192.168.1.x:9092  # Pi 4 Redpanda
POSTGIS_URL=jdbc:postgresql://localhost:5432/postgres
POSTGIS_USER=postgres
POSTGIS_PASSWORD=postgres
DELTA_LAKE_PATH=/data/delta
```

---

## Current status

⬜ Not yet started. Module renamed from ais-filter.
   ais-filter was a stateless coarse Java filter designed for
   Hetzner — that architecture is superseded.
   ais-analyser is a complete rewrite as a Spark batch job.
   