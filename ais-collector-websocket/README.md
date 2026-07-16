# ais-collector-websocket

WebSocket client for streaming AIS data sources. Currently dormant.

---

## Status

**Dormant.** aisstream.io — the service this module was built for —
went dark in March 2026. Connections stay open but no messages are
delivered. The maintainers have not responded to GitHub issues.

The module is kept intact rather than deleted because:

- A future free WebSocket AIS feed may emerge
- The WebSocket client infrastructure (OkHttp, auto-reconnect,
  pluggable source interface) is reusable for any similar source
- It documents the original architecture for historical context

The active data source is now the self-operated RTL-SDR field
receiver on Troodos, forwarding NMEA via
[ais-collector-nmea](../ais-collector-nmea/README.md).

---

## Original purpose

Consumed the aisstream.io WebSocket feed — a free crowdsourced
global AIS stream — and produced raw JSON messages to Redpanda.

aisstream.io was attractive because it required no hardware and
provided near-global coverage via a volunteer receiver network.
Its failure in early 2026 was the primary driver toward the
self-operated receiver architecture.

---

## Architecture (when active)

```
aisstream.io WebSocket
    → OkHttp WebSocket client
    → auto-reconnect (aisstream disconnects every ~2 minutes)
    → defensive JSON deserialisation
      (aisstream API was beta/unstable — unknown fields tolerated)
    → raw JSON string → Redpanda: ais.aisstream
```

---

## If a new stream source appears

To reactivate this module for a new WebSocket AIS source:

1. Implement `AisMessageSource` with the new endpoint and
   authentication
2. Update the bootstrap servers and topic name in config
3. Verify the JSON message format and update field mappings
   in `AisMessageHandler` if needed
4. The auto-reconnect, pluggable source, and Kafka production
   logic requires no changes

The pluggable `AisMessageSource` interface was designed
specifically to make source substitution straightforward.

---

## Do not depend on this module

Do not build any downstream logic that assumes this module is
running. The ais-collector-nmea module is the primary and
currently only active data source.
