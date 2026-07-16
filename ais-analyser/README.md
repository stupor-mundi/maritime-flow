Reads ais.aisstream, applies coarse stateless filtering
    Drops redundant stationary positions (moved < ~50m, speed 
    zero, no course change)
    Collapses repeated identical anchor positions
    Writes reduced stream to ais.coarse (7 day retention)
    Runs on Hetzner alongside ais-collector-websocket
    No proximity logic — intentionally simple and lightweight
