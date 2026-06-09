package io.maritimeflow.collector;

import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import okhttp3.OkHttpClient;

import java.util.concurrent.TimeUnit;

/**
 * Entry point for the exploratory AIS collector.
 * Connects to aisstream.io and prints every received message to stdout.
 *
 * Required environment variable:
 *   AISSTREAM_API_KEY — aisstream.io API key
 */
public class AisCollectorMain {

    public static void main(String[] args) throws InterruptedException {
        String apiKey = System.getenv("AISSTREAM_API_KEY");
        if (apiKey == null || apiKey.isBlank()) {
            System.err.println("ERROR: AISSTREAM_API_KEY environment variable is not set.");
            System.exit(1);
        }

        ObjectMapper mapper = new ObjectMapper()
                .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false)
                .enable(SerializationFeature.INDENT_OUTPUT);

        OkHttpClient client = new OkHttpClient.Builder()
                .pingInterval(20, TimeUnit.SECONDS)
                .build();

        AisStreamWebSocketSource source = new AisStreamWebSocketSource(client, mapper, apiKey);
        source.connect();

        // Park the main thread — all work happens on OkHttp and scheduler threads.
        Thread.currentThread().join();
    }
}
