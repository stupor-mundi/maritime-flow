# ais-collector-nmea

Receives decoded NMEA sentences from the field node phone via UDP
and produces them as raw strings to Redpanda. The first module in
the maritime-flow pipeline and the simplest — its only job is to
be a reliable UDP-to-Kafka bridge running continuously on the Pi 4.

Written in Go for a single-binary deployment with no JVM dependency
on the Pi 4.

---

## What it does

```
Field phone (AIS-catcher for Android)
    → UDP port 10110 via mobile data + DuckDNS
    → ais-collector-nmea
        validates NMEA format and checksum
        extracts MMSI for Kafka key
        produces raw NMEA string to Redpanda: ais.nmea
```

It does not decode, parse, or filter messages — it preserves the
raw NMEA string exactly as received. All analytical work happens
downstream in ais-analyser.

---

## Why Go

The Pi 4 runs this as an always-on systemd service. A Go binary:

- Compiles to a single statically-linked executable (~5MB)
- Uses ~10-15MB RAM at runtime — no JVM overhead
- Starts in milliseconds — fast restart on watchdog trigger
- Cross-compiles easily: build on any machine, deploy to Pi 4 ARM64
- No runtime to install on the Pi — copy binary, done

---

## Structure

```
ais-collector-nmea/
├── main.go               entry point, UDP socket loop
├── nmea_validator.go     checksum verification + format check
├── mmsi_extractor.go     extracts MMSI from NMEA sentence
├── kafka_producer.go     Kafka client → Redpanda ais.nmea
├── config.go             configuration from env vars
├── go.mod
└── README.md
```

---

## Kafka topic

```
Topic:      ais.nmea
Key:        MMSI (UTF-8 string)
Value:      raw NMEA sentence (UTF-8 string)
Retention:  indefinite — this is the raw archive
            all downstream processing reads from here
```

Keying by MMSI ensures all messages for the same vessel land on the
same Kafka partition, which matters for ordered processing in
ais-analyser.

---

## NMEA validation

Valid NMEA sentences start with `!` or `$` and end with a checksum
after `*`. Example:

```
!AIVDM,1,1,,A,13HOI:0P0000000000000000000,0*49
```

Validation rejects:
- Sentences not starting with `!` or `$`
- Sentences with invalid checksum
- Empty or blank messages

Invalid messages are logged as warnings and dropped — they are never
produced to Kafka.

---

## MMSI extraction

MMSI is extracted from the decoded AIS payload in the NMEA sentence.
For AIVDM/AIVDO sentences, the payload is in field 6 (0-indexed).
The MMSI occupies bits 8-37 of the decoded payload.

If MMSI cannot be extracted (malformed payload, unsupported message
type), the message is produced with an empty key and a warning is
logged. It is not dropped — the raw data is preserved.

---

## Configuration

All configuration via environment variables:

```bash
UDP_PORT=10110                        # port to listen on
KAFKA_BOOTSTRAP_SERVERS=localhost:9092 # Redpanda address
KAFKA_TOPIC=ais.nmea                  # topic to produce to
LOG_LEVEL=info                        # debug|info|warn|error
```

---

## Building

```bash
# Build for current machine
go build -o ais-collector-nmea .

# Cross-compile for Pi 4 (ARM64 Linux)
GOOS=linux GOARCH=arm64 go build -o ais-collector-nmea .

# Deploy to Pi 4
scp ais-collector-nmea pi@yourpi.local:/opt/ais-collector-nmea
```

---

## systemd service

Copy the binary to `/opt/` and install the service:

```ini
# /etc/systemd/system/ais-collector-nmea.service

[Unit]
Description=AIS NMEA Collector
After=network.target redpanda.service

[Service]
ExecStart=/opt/ais-collector-nmea
Restart=always
RestartSec=5
Environment=UDP_PORT=10110
Environment=KAFKA_BOOTSTRAP_SERVERS=localhost:9092
Environment=KAFKA_TOPIC=ais.nmea
Environment=LOG_LEVEL=info

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable ais-collector-nmea
sudo systemctl start ais-collector-nmea

# Check logs
journalctl -u ais-collector-nmea -f
```

---

## MQTT health metrics

In addition to producing NMEA to Redpanda, ais-collector-nmea
publishes health metrics to the MQTT broker on Pi 4:

```
field/troodos/health    message rate (msgs/min)
                        last message timestamp
                        Kafka producer error count
                        uptime
```

Published every 60 seconds. Mosquitto must be running on Pi 4.
If MQTT is unavailable, health publishing is skipped silently —
NMEA production to Redpanda continues unaffected.

---

## Field node setup (AIS-catcher)

Configure AIS-catcher for Android to forward NMEA to the Pi 4:

```
Destination: yourname.duckdns.org:10110 (UDP)
```

The DuckDNS hostname resolves to the current home IP regardless
of which flat the Pi 4 is in. No changes needed on the phone or
field node when the Pi 4 moves to a new location.

---

## Current status

⬜ Not yet written — first coding task in maritime-flow.