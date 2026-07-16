# maritime-flow

A maritime AIS tracking and intelligence system for the Eastern
Mediterranean, built around a self-operated field receiver on Troodos,
Cyprus.

Receives AIS position data from vessels transiting the Eastern Med,
detects anomalous behaviour, and infers trade flows. Focuses on
cargo vessels, tankers, and vessels of geopolitical interest in the
Suez corridor, Levant approaches, and Cyprus coastal waters.

Built as a companion learning project to [air-cargo](../air-cargo),
specifically for Apache Spark, SedonaSQL, Delta Lake, Go, and IoT/MQTT.

---

## What it does

A field receiver on an elevated Troodos slope (~1,100-1,330m) captures
AIS radio transmissions from vessels up to ~155-166km away, covering
the main Eastern Mediterranean shipping lanes without any third-party
data dependency. Decoded NMEA sentences are forwarded via mobile data
to a Pi 4 home server, which accumulates them as a raw archive.
Periodic batch analysis runs on a local Minisforum workstation using
Spark and SedonaSQL, writing results to Delta Lake.

The analytical goal is to identify vessels that behave unusually —
going dark in covered areas, appearing at unexpected ports, conducting
ship-to-ship transfers, or routing inconsistently with their declared
cargo — and to infer trade flow patterns from observed vessel movements.

---

## Architecture

```
FIELD NODE (Troodos, Cyprus, ~1,100-1,330m elevation):
  BraX2 phone + RTL-SDR V4 (USB OTG)
  AIS-catcher for Android — decodes AIS on both VHF channels
  Mobile data SIM (CYTA)
    → AISHub (cooperative feed, global data in return)
    → Pi 4 at home via DuckDNS (own pipeline)
  Flexible solar panel + LiFePO4 power bank
  MQTT: receives commands, sends health metrics + snapshots

HOME BASE (Raspberry Pi 4, 8GB, always on):
  ais-collector-nmea (Go, systemd)
    → listens UDP port 10110
    → produces raw NMEA to Redpanda: ais.nmea
  Mosquitto MQTT broker — field node command/control
  DuckDNS — stable hostname despite dynamic home IP
  Wireguard VPN — remote access from anywhere
  USB SSD (2TB) — long-term raw NMEA archive

ANALYSIS (Minisforum CachyOS, run on demand):
  Spark cluster via K3s Spark Operator
  Reads accumulated NMEA from Pi 4 Redpanda
  SedonaSQL spatial operations
  Writes analytical results to Delta Lake
  DuckDB for ad-hoc queries on Delta Lake
```

---

## Why these technology choices

**Go for ais-collector-nmea over Java**
The Pi 4 runs always-on. A Go binary uses ~10-15MB RAM and starts in
milliseconds with no JVM warmup. A single statically-linked binary is
copied to the Pi and managed by systemd — no runtime to install, no
dependency management on the target machine.

**Spark over Flink**
maritime-flow is a batch analytical system, not a real-time streaming
system. Data accumulates in Redpanda on the Pi 4 while the Minisforum
is off. When analysis is wanted, Spark processes the accumulated data
in batch. Flink's streaming strengths — stateful per-key processing,
continuous output, sub-second latency — are not needed here.

The companion project air-cargo uses Flink for stateful per-aircraft
streaming, where it is genuinely the right choice. The two projects
are deliberately complementary:

```
air-cargo:       Flink streaming → Iceberg
maritime-flow:   Spark batch → Delta Lake
```

Both use Apache Sedona, through SedonaFlink and SedonaSQL respectively.

**Delta Lake over Iceberg**
Spark is Delta Lake's primary engine — MERGE, OPTIMIZE, ZORDER, and
time travel are all first-class in the Spark/Delta combination.
Iceberg is the better choice for Flink (as used in air-cargo).
Each project uses the lakehouse format that fits its compute engine.

**Own receiver over third-party feeds**
aisstream.io — the original planned data source — went dark in March
2026. MarineTraffic was acquired by Kpler and is now enterprise-only.
A self-operated receiver at ~1,330m on Troodos covers ~166km of the
Eastern Mediterranean with no third-party dependency, no API costs,
and no service that can disappear. The data is first-party.

**Pi 4 as always-on accumulator, Minisforum on demand**
Raw NMEA is stored indefinitely on the Pi 4. The Minisforum is a
powerful analysis workstation but does not need to be always on.
When analysis is wanted, it reads from Pi 4 Redpanda and processes
whatever has accumulated. Algorithm improvements can be applied
retroactively to months of historical raw data — the raw archive
is ground truth that never changes.

**MQTT for field node control**
The field node needs bidirectional communication — not just sending
NMEA but receiving commands (take photo, reboot, status report) and
sending health metrics. MQTT is the standard IoT protocol for this
pattern. Mosquitto on Pi 4 serves as the broker. MacroDroid on the
Android phone handles all automations without custom code.

---

## Modules

| Module | Description |
|---|---|
| [ais-collector-nmea](ais-collector-nmea/README.md) | Go — UDP NMEA listener on Pi 4, produces to Redpanda |
| [ais-collector-websocket](ais-collector-websocket/README.md) | Java — dormant WebSocket client, kept for future stream sources |
| [ais-analyser](ais-analyser/README.md) | Spark + SedonaSQL batch analytical engine |
| [common](common/README.md) | Shared models, NMEA parsing, MMSI handling |
| [reference-data](reference-data/README.md) | PostGIS DDL — ports (WPI), anchorages (OSM), coastline, coverage zones, airspace restrictions |
| [k8s](k8s/README.md) | K3s manifests for Spark Operator on Minisforum |

---

## Prerequisites

**Field node hardware:**
- Android phone with USB OTG support
- RTL-SDR Blog V4 (~€35)
- VHF antenna tuned to 162MHz
- Mobile data SIM
- LiFePO4 power bank (~20,000mAh)
- Flexible solar panel (~10W, narrow format)

**Home infrastructure:**
- Raspberry Pi 4 (any RAM, 4GB+ recommended)
- USB SSD (2TB recommended for long-term archive)
- DuckDNS account (free)
- Go 1.22+

**Analysis workstation:**
- Minisforum or equivalent x86 machine
- K3s installed
- Java 17, Maven 3.8+

**Software:**
- AIS-catcher for Android (sideloaded APK)
- MacroDroid (Android automation)
- Mosquitto (MQTT broker on Pi 4)
- Redpanda (Kafka-compatible broker on Pi 4)
- Wireguard (VPN on Pi 4 and client machines)
- DuckDB (for ad-hoc Delta Lake queries)
- QGIS (for coverage polygon editing and visualisation)

---

## Getting started

**1. Set up Pi 4 home base:**
```bash
# Install Redpanda (systemd, not Docker)
# See: https://docs.redpanda.com/current/deploy/deployment-option/self-hosted/linux/

# Install Mosquitto
sudo apt install mosquitto mosquitto-clients

# Install Wireguard
sudo apt install wireguard

# Install DuckDNS cron
# See: https://www.duckdns.org/install.jsp

# Create Kafka topic
rpk topic create ais.nmea --retention-ms=-1
```

**2. Build and deploy ais-collector-nmea:**
```bash
cd ais-collector-nmea
go build -o ais-collector-nmea .
# Cross-compile for Pi 4 (ARM64) from another machine:
GOOS=linux GOARCH=arm64 go build -o ais-collector-nmea .
scp ais-collector-nmea pi@home.duckdns.org:/opt/
# Install systemd service — see ais-collector-nmea/README.md
```

**3. Configure AIS-catcher on the field phone:**
```
Destination 1: AISHub UDP endpoint
Destination 2: yourname.duckdns.org:10110
```

**4. Load reference data into PostGIS:**
```bash
cd reference-data/scripts
# Loads: ports (World Port Index), anchorages (OSM),
#        coastline (Natural Earth)
# Coverage zones and airspace restrictions drawn in QGIS
# See reference-data/README.md for full instructions
```

**5. Run analysis:**
```bash
cd ais-analyser
# See ais-analyser/README.md for Spark job submission
```

---

## Current state

| Component | Status |
|---|---|
| Field node hardware | ⬜ IRL recon pending — Troodos site selection |
| Pi 4 home base | ⬜ OS install and services pending |
| ais-collector-nmea | ⬜ not yet written — first coding task |
| ais-collector-websocket | ✅ exists, dormant (aisstream.io dead) |
| ais-analyser | ⬜ not yet started |
| reference-data DDL | ⬜ partial — coverage/airspace polygons pending |
| K3s Spark deployment | ⬜ not yet configured |
| Delta Lake | ⬜ not yet implemented |
| AISHub registration | ⬜ pending field node deployment |

---

## Data sources

- **Self-operated RTL-SDR receiver** — primary, first-party AIS data
- **AISHub** — cooperative network, global feed in exchange for
  contributing local receiver data
- **World Port Index (WPI)** — US NGA, free download, global port
  reference data: coordinates, port type, facilities
- **OpenStreetMap** — anchorage polygons, coastline, harbour areas
- **Natural Earth** — coastline for land/sea boundary (spoofing detection)
- **Hand-drawn polygons in QGIS** — coverage zones, airspace
  restrictions, sensitive/grey-market locations → PostGIS

---

## Companion project

[air-cargo](../air-cargo) — ADS-B flight tracking using Flink
streaming. The two projects share spatial analysis goals and PostGIS
infrastructure but use deliberately different compute stacks
(Flink vs Spark, Iceberg vs Delta Lake) for comparative learning.

---

## License

MIT

