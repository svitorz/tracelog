// Command loggen generates the synthetic log files of examples/logs (spec 0004, seção 4.6).
//
// The output is deterministic: the same flags always produce byte-identical files. All
// data is fictitious: a made-up "checkout-api", users user-NNNN and IP addresses only
// from the documentation ranges of RFC 5737 (192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24).
//
//	go run ./tools/loggen -out examples/logs            # the versioned dataset
//	go run ./tools/loggen -scale 200 -out /tmp/biglogs  # larger volume, not versioned
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type config struct {
	Seed  uint64
	Start time.Time
	Days  int
	Scale int
}

func defaultConfig() config {
	return config{
		Seed:  42,
		Start: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Days:  14,
		Scale: 1,
	}
}

// Scenario days, as offsets from Start (with the defaults: 2026-03-06 and 2026-03-10).
const (
	credentialStuffingDay = 5
	databaseOutageDay     = 9
)

func main() {
	cfg := defaultConfig()
	out := flag.String("out", "examples/logs", "output directory")
	start := flag.String("start", cfg.Start.Format("2006-01-02"), "first day (YYYY-MM-DD, UTC)")
	flag.Uint64Var(&cfg.Seed, "seed", cfg.Seed, "random seed")
	flag.IntVar(&cfg.Days, "days", cfg.Days, "number of days")
	flag.IntVar(&cfg.Scale, "scale", cfg.Scale, "volume multiplier")
	flag.Parse()

	day, err := time.Parse("2006-01-02", *start)
	if err != nil {
		log.Fatalf("invalid -start: %v", err)
	}
	cfg.Start = day
	if cfg.Days < 1 || cfg.Scale < 1 {
		log.Fatal("-days and -scale must be at least 1")
	}

	files := generate(cfg)
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(*out, name)
		if err := os.WriteFile(path, files[name], 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s: %d lines\n", path, bytes.Count(files[name], []byte("\n")))
	}
}

type line struct {
	at   time.Time
	text string
}

type generator struct {
	cfg config
	rng *rand.Rand
}

// generate returns the content of each file, keyed by file name.
func generate(cfg config) map[string][]byte {
	g := &generator{cfg: cfg, rng: rand.New(rand.NewPCG(cfg.Seed, 0x7ace10))}

	// Order matters for determinism: each file consumes the random stream in turn.
	return map[string][]byte{
		"checkout-api.log":   g.checkoutAPI(),
		"auth-service.log":   g.authService(),
		"legacy-gateway.log": g.legacyGateway(),
	}
}

// checkoutAPI writes one JSON object per line.
func (g *generator) checkoutAPI() []byte {
	var lines []line
	for d := 0; d < g.cfg.Days; d++ {
		day := g.cfg.Start.AddDate(0, 0, d)

		for i := 0; i < 12*g.cfg.Scale; i++ {
			at := g.timeIn(day, 8, 20)
			lines = append(lines, line{at, g.checkoutEvent(at)})
		}

		// Background noise: a couple of errors and warnings every day.
		for i := 0; i < g.rng.IntN(3)*g.cfg.Scale; i++ {
			at := g.timeIn(day, 8, 20)
			lines = append(lines, line{at, jsonLine(map[string]any{
				"timestamp": at.Format(time.RFC3339), "level": "error",
				"message": "payment provider timeout", "trace_id": g.traceID(), "provider": "acme-pay",
			})})
		}

		if d == databaseOutageDay {
			// Error spike: the database goes away for 90 minutes.
			outage := day.Add(14 * time.Hour)
			for i := 0; i < 25*g.cfg.Scale; i++ {
				at := outage.Add(time.Duration(g.rng.IntN(90*60)) * time.Second)
				lines = append(lines, line{at, jsonLine(map[string]any{
					"timestamp": at.Format(time.RFC3339), "level": "error",
					"message": "database connection refused", "trace_id": g.traceID(), "db_host": "orders-db",
				})})
			}
			at := outage.Add(47 * time.Minute)
			lines = append(lines, line{at, jsonLine(map[string]any{
				"timestamp": at.Format(time.RFC3339), "level": "fatal",
				"message": "order worker crashed: connection pool exhausted", "trace_id": g.traceID(),
			})})
		}

		if d%4 == 2 {
			// A line cut in the middle, as when a process dies while writing.
			at := g.timeIn(day, 8, 20)
			full := jsonLine(map[string]any{"timestamp": at.Format(time.RFC3339), "level": "info", "message": "order placed", "order_id": g.orderID()})
			lines = append(lines, line{at, full[:len(full)/2]})
		}

		if d%5 == 1 {
			// Valid JSON without a timestamp: imported with received_at as occurred_at.
			lines = append(lines, line{day.Add(23 * time.Hour), jsonLine(map[string]any{"level": "info", "message": "cache warmed", "entries": 128 + g.rng.IntN(512)})})
		}
	}

	return render(lines)
}

func (g *generator) checkoutEvent(at time.Time) string {
	fields := map[string]any{
		"timestamp": at.Format(time.RFC3339),
		"trace_id":  g.traceID(),
		"user":      g.user(),
	}

	switch n := g.rng.IntN(100); {
	case n < 45:
		fields["level"], fields["message"] = "info", "order placed"
		fields["order_id"], fields["amount_cents"] = g.orderID(), 500+g.rng.IntN(49500)
	case n < 75:
		fields["level"], fields["message"] = "info", "payment authorized"
		fields["order_id"] = g.orderID()
	case n < 88:
		fields["level"], fields["message"] = "info", "order shipped"
		fields["order_id"] = g.orderID()
	case n < 95:
		fields["level"], fields["message"] = "warn", "slow query detected"
		fields["duration_ms"] = 800 + g.rng.IntN(2200)
	default:
		fields["level"], fields["message"] = "warn", "retrying payment provider request"
		fields["retry_count"] = 1 + g.rng.IntN(3)
	}

	return jsonLine(fields)
}

// authService writes "timestamp LEVEL message" lines.
func (g *generator) authService() []byte {
	var lines []line
	for d := 0; d < g.cfg.Days; d++ {
		day := g.cfg.Start.AddDate(0, 0, d)

		for i := 0; i < 8*g.cfg.Scale; i++ {
			at := g.timeIn(day, 7, 22)
			lines = append(lines, line{at, fmt.Sprintf("%s INFO user login succeeded user=%s ip=%s", at.Format(time.RFC3339), g.user(), g.clientIP())})
		}

		for i := 0; i < (1+g.rng.IntN(2))*g.cfg.Scale; i++ {
			at := g.timeIn(day, 7, 22)
			lines = append(lines, line{at, fmt.Sprintf("%s WARN login failed for %s ip=%s reason=invalid_password", at.Format(time.RFC3339), g.user(), g.clientIP())})
		}

		if d == credentialStuffingDay {
			// Credential stuffing: many failed logins from one /24 in a few minutes.
			burst := day.Add(2*time.Hour + 10*time.Minute)
			for i := 0; i < 40*g.cfg.Scale; i++ {
				at := burst.Add(time.Duration(g.rng.IntN(15*60)) * time.Second)
				lines = append(lines, line{at, fmt.Sprintf("%s WARN login failed for %s ip=%s reason=invalid_password", at.Format(time.RFC3339), g.user(), g.attackerIP())})
			}
			at := burst.Add(16 * time.Minute)
			lines = append(lines, line{at, fmt.Sprintf("%s ERROR unauthorized access attempt to /admin ip=%s", at.Format(time.RFC3339), g.attackerIP())})
			// Same burst seen by a proxy with a timestamp format the importer does not know.
			lines = append(lines, line{at.Add(time.Second), fmt.Sprintf("[%s] WARN account locked for %s after repeated login failures", at.Add(time.Second).Format("02/Jan/2006:15:04:05 -0700"), g.user())})
		}
	}

	return render(lines)
}

// legacyGateway writes free text without timestamps, plus a few broken lines.
func (g *generator) legacyGateway() []byte {
	messages := []string{
		"gateway started on port 9000",
		"upstream connection reset by peer",
		"health probe ok",
		"config reloaded from /etc/gateway.conf",
		"GC pause 120ms",
	}

	var lines []line
	for d := 0; d < g.cfg.Days; d++ {
		day := g.cfg.Start.AddDate(0, 0, d)
		for i := 0; i < 3*g.cfg.Scale; i++ {
			lines = append(lines, line{day.Add(time.Duration(i) * time.Minute), messages[g.rng.IntN(len(messages))]})
		}
		if d%3 == 0 {
			lines = append(lines, line{day.Add(time.Hour), ""})
		}
		if d%7 == 3 {
			lines = append(lines, line{day.Add(2 * time.Hour), "~~~ 4f 3a 00 1b ~~~"})
		}
	}

	return render(lines)
}

// render sorts lines by time (stable, so equal times keep generation order).
func render(lines []line) []byte {
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].at.Before(lines[j].at) })

	var buf bytes.Buffer
	for _, l := range lines {
		buf.WriteString(l.text)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func (g *generator) timeIn(day time.Time, fromHour, toHour int) time.Time {
	span := (toHour - fromHour) * 3600
	return day.Add(time.Duration(fromHour)*time.Hour + time.Duration(g.rng.IntN(span))*time.Second)
}

func (g *generator) user() string       { return fmt.Sprintf("user-%04d", 1+g.rng.IntN(60)) }
func (g *generator) orderID() string    { return fmt.Sprintf("ord-%06d", g.rng.IntN(1_000_000)) }
func (g *generator) traceID() string    { return fmt.Sprintf("%016x", g.rng.Uint64()) }
func (g *generator) attackerIP() string { return fmt.Sprintf("203.0.113.%d", 1+g.rng.IntN(254)) }

func (g *generator) clientIP() string {
	if g.rng.IntN(2) == 0 {
		return fmt.Sprintf("192.0.2.%d", 1+g.rng.IntN(254))
	}
	return fmt.Sprintf("198.51.100.%d", 1+g.rng.IntN(254))
}

// jsonLine encodes fields with sorted keys (encoding/json sorts map keys).
func jsonLine(fields map[string]any) string {
	b, err := json.Marshal(fields)
	if err != nil {
		panic(err) // only static, encodable values are used
	}
	return string(b)
}
