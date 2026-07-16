# ais-tools

Standalone Go CLI tools for developing and testing the maritime-flow
pipeline without live AIS traffic. Stdlib only, no external
dependencies.

## Tools

### ais-replayer

Replays historical AIS CSV data as NMEA AIVDM sentences over UDP,
simulating the field node phone (AIS-catcher for Android) that
normally forwards live traffic to ais-collector-nmea.

Build:

    go build -o ais-replayer ./cmd/ais-replayer

Usage:

    ./ais-replayer --file historical.csv --host localhost --port 10110 --speed 10.0

Flags:

    --file    path to input CSV file (required)
    --host    UDP destination host (default "localhost")
    --port    UDP destination port (default "10110")
    --speed   replay speed multiplier, 10.0 = 10x realtime,
              0.1 = slow motion (default 1.0)
    --loop    loop the file when exhausted (default false)
    --format  input format: "dma" (default) - "noaa" is accepted
              but not yet implemented

Input data: Danish Maritime Authority historical AIS data
https://web.ais.dk/aisdata/

## Output compatibility

ais-replayer produces the same UDP NMEA AIVDM stream that the field
node phone sends. The rest of the pipeline (ais-collector-nmea
onward) requires no changes to consume replayed data.
