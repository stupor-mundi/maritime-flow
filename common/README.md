# common

Shared models, constants, and utilities used across maritime-flow
Java modules. Infrastructure-independent — contains no Kafka, Spark,
or PostGIS dependencies.

---

## Contents

```
common/
└── src/main/java/io/aircargo/maritime/common/
    ├── model/
    │   ├── AisMessage.java          raw decoded NMEA message
    │   ├── PositionReport.java      Class A/B position report (msg 1,2,3,18)
    │   ├── ShipStaticData.java      static voyage data (msg 5,24)
    │   └── VesselState.java         aggregated per-vessel state
    ├── nmea/
    │   ├── NmeaParser.java          NMEA sentence → AisMessage
    │   ├── NmeaValidator.java       checksum + format validation
    │   └── MmsiExtractor.java       MMSI from NMEA payload
    ├── constants/
    │   ├── KafkaTopics.java         topic name constants
    │   └── Geographic.java          coordinate system constants
    └── enums/
        ├── VesselType.java          AIS vessel type codes + max speed
        ├── NavigationStatus.java    underway, anchored, moored, etc.
        └── MessageType.java         AIS message type 1-27
```

---

## Key design notes

**NMEA parsing**

AIS messages arrive as NMEA 0183 sentences — text strings starting
with `!AIVDM` or `!AIVDO`. Multi-part messages (large payloads split
across multiple sentences) must be reassembled before decoding the
6-bit encoded payload.

MMSI is 9 digits and must be treated as a string — leading zeros
are significant and must be preserved. Never store MMSI as an integer.

**Defensive deserialisation**

AIS field population is unreliable — many fields are optional or
frequently zero/null even when mandatory. All model fields are
nullable. Callers must handle null values throughout.

**VesselType enum**

Includes `maxSpeedKnots` per vessel type for physical plausibility
checks in ais-analyser. A tanker doing 40 knots is a spoofing signal
regardless of what the AIS message says.

**Coordinate order**

AIS position reports encode longitude then latitude — the opposite
of the common lat/lon convention. All model classes store fields
in the order `latitude, longitude` for clarity. Construction of
geometry points must explicitly use `(longitude, latitude)` order.

---

## Dependencies

Minimal by design:

```xml
<dependency>
    <groupId>com.fasterxml.jackson.core</groupId>
    <artifactId>jackson-databind</artifactId>
</dependency>
```

No Kafka, no Spark, no PostGIS. Modules that need those add them
as their own dependencies.

---

## Current status

⬜ Models exist from original ais-collector development.
   Review needed — field names and types may need updating
   to reflect current architecture and NMEA-first approach.
   