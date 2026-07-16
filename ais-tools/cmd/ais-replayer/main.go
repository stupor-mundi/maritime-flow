package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type config struct {
	File   string
	Host   string
	Port   string
	Speed  float64
	Loop   bool
	Format string
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.File, "file", "", "path to input CSV file (required)")
	flag.StringVar(&cfg.Host, "host", "localhost", "UDP destination host")
	flag.StringVar(&cfg.Port, "port", "10110", "UDP destination port")
	flag.Float64Var(&cfg.Speed, "speed", 1.0, "replay speed multiplier (10.0 = 10x realtime, 0.1 = slow motion)")
	flag.BoolVar(&cfg.Loop, "loop", false, "loop the file when exhausted")
	flag.StringVar(&cfg.Format, "format", "dma", `input format: "dma" or "noaa"`)
	flag.Parse()

	if cfg.File == "" {
		fmt.Fprintln(os.Stderr, "FATAL: --file is required")
		os.Exit(1)
	}
	if cfg.Format != "dma" {
		fmt.Fprintf(os.Stderr, "FATAL: --format=%q is not yet implemented (only \"dma\" is supported)\n", cfg.Format)
		os.Exit(1)
	}

	return cfg
}

type record struct {
	Timestamp time.Time
	MMSI      uint32
	Latitude  float64
	Longitude float64
	NavStatus int
	ROT       int
	SOG       float64
	COG       float64
	Heading   int
}

func loadDmaCsv(path string) ([]record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(bufio.NewReader(f))
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	idx := make(map[string]int, len(header))
	for i, name := range header {
		idx[strings.TrimSpace(name)] = i
	}

	required := []string{"Timestamp", "MMSI", "Latitude", "Longitude", "Navigational_status", "ROT", "SOG", "COG", "Heading"}
	for _, name := range required {
		if _, ok := idx[name]; !ok {
			return nil, fmt.Errorf("missing required column %q", name)
		}
	}

	var records []record
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Warn("skipping malformed CSV row", "error", err)
			continue
		}

		ts, err := time.Parse("02/01/2006 15:04:05", strings.TrimSpace(row[idx["Timestamp"]]))
		if err != nil {
			slog.Warn("skipping row with unparseable timestamp", "value", row[idx["Timestamp"]])
			continue
		}

		mmsiVal, err := strconv.ParseUint(strings.TrimSpace(row[idx["MMSI"]]), 10, 32)
		if err != nil {
			slog.Warn("skipping row with unparseable MMSI", "value", row[idx["MMSI"]])
			continue
		}

		lat, err := strconv.ParseFloat(strings.TrimSpace(row[idx["Latitude"]]), 64)
		if err != nil {
			slog.Warn("skipping row with unparseable latitude", "value", row[idx["Latitude"]])
			continue
		}

		lon, err := strconv.ParseFloat(strings.TrimSpace(row[idx["Longitude"]]), 64)
		if err != nil {
			slog.Warn("skipping row with unparseable longitude", "value", row[idx["Longitude"]])
			continue
		}

		heading := 511
		if h, err := strconv.ParseFloat(strings.TrimSpace(row[idx["Heading"]]), 64); err == nil {
			heading = int(h)
		}

		rot := -128
		if r, err := strconv.ParseFloat(strings.TrimSpace(row[idx["ROT"]]), 64); err == nil {
			rot = int(r)
		}

		records = append(records, record{
			Timestamp: ts,
			MMSI:      uint32(mmsiVal),
			Latitude:  lat,
			Longitude: lon,
			NavStatus: mapNavStatus(row[idx["Navigational_status"]]),
			ROT:       rot,
			SOG:       parseFloatOr(row[idx["SOG"]], 0),
			COG:       parseFloatOr(row[idx["COG"]], 0),
			Heading:   heading,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	return records, nil
}

func mapNavStatus(s string) int {
	switch strings.TrimSpace(s) {
	case "Under way using engine":
		return 0
	case "At anchor":
		return 1
	case "Moored":
		return 5
	default:
		return 15
	}
}

func parseFloatOr(s string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return fallback
	}
	return v
}

// --- NMEA AIVDM Type 1 (Class A position report) encoding ---

type bitWriter struct {
	bits []byte
}

func (w *bitWriter) writeUint(value uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		w.bits = append(w.bits, byte((value>>uint(i))&1))
	}
}

func (w *bitWriter) writeInt(value int64, n int) {
	mask := uint64(1)<<uint(n) - 1
	w.writeUint(uint64(value)&mask, n)
}

func sixBitToChar(v byte) byte {
	if v < 40 {
		return v + 48
	}
	return v + 56
}

func (w *bitWriter) toPayload() string {
	var sb strings.Builder
	for i := 0; i < len(w.bits); i += 6 {
		var v byte
		for j := 0; j < 6; j++ {
			v = v<<1 | w.bits[i+j]
		}
		sb.WriteByte(sixBitToChar(v))
	}
	return sb.String()
}

func encodeType1(r record) string {
	sog10 := clamp(int(math.Round(r.SOG*10)), 0, 1023)
	cog10 := clamp(int(math.Round(r.COG*10)), 0, 4095)
	heading := r.Heading
	if heading != 511 {
		heading = clamp(heading, 0, 359)
	}
	tsSecond := clamp(r.Timestamp.Second(), 0, 59)

	w := &bitWriter{}
	w.writeUint(1, 6)                                     // message type
	w.writeUint(0, 2)                                      // repeat indicator
	w.writeUint(uint64(r.MMSI), 30)                        // MMSI
	w.writeUint(uint64(r.NavStatus), 4)                    // navigational status
	w.writeInt(int64(r.ROT), 8)                            // rate of turn
	w.writeUint(uint64(sog10), 10)                         // speed over ground
	w.writeUint(0, 1)                                      // position accuracy
	w.writeInt(int64(math.Round(r.Longitude*600000)), 28)  // longitude
	w.writeInt(int64(math.Round(r.Latitude*600000)), 27)   // latitude
	w.writeUint(uint64(cog10), 12)                         // course over ground
	w.writeUint(uint64(heading), 9)                        // true heading
	w.writeUint(uint64(tsSecond), 6)                       // timestamp (UTC second)
	w.writeUint(0, 2)                                      // maneuver indicator
	w.writeUint(0, 3)                                      // spare
	w.writeUint(0, 1)                                      // RAIM flag
	w.writeUint(0, 19)                                     // radio status

	return w.toPayload()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func buildSentence(payload string) string {
	body := fmt.Sprintf("AIVDM,1,1,,A,%s,0", payload)

	var checksum byte
	for i := 0; i < len(body); i++ {
		checksum ^= body[i]
	}

	return fmt.Sprintf("!%s*%02X", body, checksum)
}

// --- main ---

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cfg := parseFlags()

	records, err := loadDmaCsv(cfg.File)
	if err != nil {
		slog.Error("failed to load CSV", "error", err)
		os.Exit(1)
	}
	if len(records) == 0 {
		slog.Error("CSV file contains no usable rows")
		os.Exit(1)
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	conn, err := net.Dial("udp4", addr)
	if err != nil {
		slog.Error("failed to dial UDP destination", "addr", addr, "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("starting replay", "file", cfg.File, "destination", addr, "speed", cfg.Speed, "loop", cfg.Loop, "records", len(records))

	var totalSent int64

	for {
		if !replayOnce(ctx, conn, records, cfg.Speed, &totalSent) {
			break
		}
		if !cfg.Loop {
			break
		}
		slog.Info("reached end of file, looping", "totalMessagesSent", totalSent)
	}

	slog.Info("replay finished", "totalMessagesSent", totalSent)
}

// replayOnce sends one full pass over records. Returns false if interrupted by a shutdown signal.
func replayOnce(ctx context.Context, conn net.Conn, records []record, speed float64, totalSent *int64) bool {
	var prevTs time.Time

	for i, r := range records {
		select {
		case <-ctx.Done():
			slog.Info("shutdown signal received", "totalMessagesSent", *totalSent)
			return false
		default:
		}

		if i > 0 {
			delay := r.Timestamp.Sub(prevTs)
			if delay > 0 && speed > 0 {
				scaled := time.Duration(float64(delay) / speed)
				select {
				case <-time.After(scaled):
				case <-ctx.Done():
					slog.Info("shutdown signal received", "totalMessagesSent", *totalSent)
					return false
				}
			}
		}
		prevTs = r.Timestamp

		sentence := buildSentence(encodeType1(r))
		if _, err := conn.Write([]byte(sentence)); err != nil {
			slog.Error("udp send failed", "error", err)
			continue
		}

		*totalSent++
		if *totalSent%1000 == 0 {
			slog.Info("replay progress", "messagesReplayed", *totalSent, "currentTimestamp", r.Timestamp)
		}
	}

	return true
}
