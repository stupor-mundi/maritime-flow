package io.maritimeflow.collector;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.WebSocket;
import okhttp3.WebSocketListener;

import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicLong;

/**
 * OkHttp WebSocket source for aisstream.io.
 * Subscribes to PositionReport and ShipStaticData messages for a given
 * bounding box, pretty-prints each message to stdout, and reconnects
 * automatically on both clean disconnects (~2min server-side) and failures.
 */
class AisStreamWebSocketSource extends WebSocketListener {

    private static final String STREAM_URL = "wss://stream.aisstream.io/v0/stream";

    private static final long CLEAN_RECONNECT_DELAY_SECONDS = 1;
    private static final long MAX_BACKOFF_SECONDS = 30;

    private final OkHttpClient client;
    private final ObjectMapper mapper;
    private final String apiKey;
    private final ScheduledExecutorService scheduler =
            Executors.newSingleThreadScheduledExecutor(r -> {
                Thread t = new Thread(r, "ais-reconnect");
                t.setDaemon(true);
                return t;
            });

    private final AtomicInteger failureStreak = new AtomicInteger(0);
    private final AtomicLong messageCount = new AtomicLong(0);
    private final AtomicBoolean counterStarted = new AtomicBoolean(false);

    AisStreamWebSocketSource(OkHttpClient client, ObjectMapper mapper, String apiKey) {
        this.client = client;
        this.mapper = mapper;
        this.apiKey = apiKey;
    }

    void connect() {
        // Start the periodic message counter once, on the first connect call.
        if (counterStarted.compareAndSet(false, true)) {
            scheduler.scheduleAtFixedRate(
                    () -> System.out.printf("[collector] Messages received: %d%n", messageCount.get()),
                    10, 10, TimeUnit.SECONDS);
        }
        System.out.println("[collector] Connecting to " + STREAM_URL + " ...");
        Request request = new Request.Builder().url(STREAM_URL).build();
        client.newWebSocket(request, this);
    }

    // --- WebSocketListener callbacks ---

    @Override
    public void onOpen(WebSocket ws, Response response) {
        failureStreak.set(0);
        // Delay subscription by 100ms to ensure the handshake is fully settled
        // before the server's 3-second subscription timeout begins counting.
        scheduler.schedule(() -> {
            String subscription = buildSubscription();
            String maskedKey = apiKey.length() > 8 ? apiKey.substring(0, 8) + "..." : "********";
            System.out.println("[collector] Sending subscription (key=" + maskedKey + "): "
                    + subscription.replaceFirst("\"Apikey\":\"[^\"]*\"", "\"Apikey\":\"" + maskedKey + "\""));
            ws.send(subscription);
        }, 100, TimeUnit.MILLISECONDS);
    }

    @Override
    public void onMessage(WebSocket ws, String text) {
        messageCount.incrementAndGet();
        try {
            JsonNode node = mapper.readTree(text);
            String msgType = node.path("MessageType").asText("unknown");
            System.out.println("--- " + msgType + " ---");
            System.out.println(mapper.writeValueAsString(node));
        } catch (Exception e) {
            // If the message is unparseable, print raw text so nothing is lost.
            System.out.println("[collector] (raw, unparseable JSON) " + text);
        }
    }

    @Override
    public void onClosing(WebSocket ws, int code, String reason) {
        ws.close(1000, null);
    }

    @Override
    public void onClosed(WebSocket ws, int code, String reason) {
        System.out.printf("[collector] Connection closed (code=%d reason='%s'). Reconnecting in %ds.%n",
                code, reason, CLEAN_RECONNECT_DELAY_SECONDS);
        scheduler.schedule(this::connect, CLEAN_RECONNECT_DELAY_SECONDS, TimeUnit.SECONDS);
    }

    @Override
    public void onFailure(WebSocket ws, Throwable t, Response response) {
        int streak = failureStreak.getAndIncrement();
        long delay = Math.min(MAX_BACKOFF_SECONDS, (long) Math.pow(2, streak));
        System.out.printf("[collector] Connection failure (attempt=%d): %s. Reconnecting in %ds.%n",
                streak + 1, t.getMessage(), delay);
        scheduler.schedule(this::connect, delay, TimeUnit.SECONDS);
    }

    // --- Subscription builder ---

    private String buildSubscription() {
        try {
            ObjectNode sub = mapper.createObjectNode();
            sub.put("APIKey", apiKey);

            // Mediterranean bounding box: [[lat_min, lon_min], [lat_max, lon_max]]
            ArrayNode point1 = mapper.createArrayNode().add(-90).add(-180);
            ArrayNode point2 = mapper.createArrayNode().add(90).add(180);

            ArrayNode box = mapper.createArrayNode().add(point1).add(point2);
            ArrayNode boxes = mapper.createArrayNode().add(box);
            sub.set("BoundingBoxes", boxes);

            ArrayNode types = mapper.createArrayNode()
                    .add("PositionReport")
                    .add("ShipStaticData");
            sub.set("FilterMessageTypes", types);

            return mapper.writeValueAsString(sub);
        } catch (Exception e) {
            throw new RuntimeException("Failed to build subscription JSON", e);
        }
    }
}
