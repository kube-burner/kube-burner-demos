package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AppConfig holds runtime options derived from environment variables.
type AppConfig struct {
	ListenAddr       string        `json:"listenAddr"`
	WorkIterations   int           `json:"workIterations"`
	WorkSleep        time.Duration `json:"workSleep"`
	BugMode          string        `json:"bugMode"`
	BugMultiplier    int           `json:"bugMultiplier"`
	LeakCapBytes     int           `json:"leakCapBytes"`
	BackgroundWorker bool          `json:"backgroundWorker"`
}

// WorkStats describes the outcome of one workload execution.
type WorkStats struct {
	Timestamp      time.Time     `json:"timestamp"`
	Duration       time.Duration `json:"duration"`
	Iterations     int           `json:"iterations"`
	BugMode        string        `json:"bugMode"`
	BugMultiplier  int           `json:"bugMultiplier"`
	AllocatedBytes int           `json:"allocatedBytes"`
	RetainedBytes  int           `json:"retainedBytes"`
	ResultChecksum float64       `json:"resultChecksum"`
}

var (
	cfg           AppConfig
	statsMu       sync.RWMutex
	lastStats     WorkStats
	leakMu        sync.Mutex
	leakChunks    [][]byte
	retainedBytes int
)

func main() {
	cfg = loadConfig()
	log.Printf("Starting pprof demo server with config: %+v", cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.BackgroundWorker {
		go workerLoop(ctx)
	} else {
		log.Print("Background worker disabled; only /work endpoint triggers activity.")
	}

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/healthz", handleHealth)
	http.HandleFunc("/work", handleWork)
	http.HandleFunc("/stats", handleStats)
	http.HandleFunc("/config", handleConfig)

	log.Printf("HTTP server listening on %s (pprof at /debug/pprof/)", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, nil); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Server exited with error: %v", err)
	}
}

func workerLoop(ctx context.Context) {
	log.Print("Background worker started")
	ticker := time.NewTicker(cfg.WorkSleep)
	defer ticker.Stop()

	for {
		stats := runWorkOnce()
		setLastStats(stats)
		select {
		case <-ctx.Done():
			log.Print("Background worker stopping")
			return
		case <-ticker.C:
		}
	}
}

func runWorkOnce() WorkStats {
	start := time.Now()
	checksum, allocBytes, retained := performComputation(cfg.WorkIterations, cfg.BugMultiplier, cfg.BugMode, cfg.LeakCapBytes)
	return WorkStats{
		Timestamp:      time.Now().UTC(),
		Duration:       time.Since(start),
		Iterations:     cfg.WorkIterations,
		BugMode:        cfg.BugMode,
		BugMultiplier:  cfg.BugMultiplier,
		AllocatedBytes: allocBytes,
		RetainedBytes:  retained,
		ResultChecksum: checksum,
	}
}

func performComputation(iterations, bugMultiplier int, bugMode string, leakCap int) (float64, int, int) {
	if iterations <= 0 {
		return 0, 0, retainedBytes
	}
	if bugMultiplier < 0 {
		bugMultiplier = 0
	}

	var checksum float64
	allocated := 0
	for i := 0; i < iterations; i++ {
		base := math.Sin(float64(i%360)) + math.Cos(float64((i*7)%360))
		checksum += math.Sqrt(math.Abs(base) + float64((i%7)+1))

		switch bugMode {
		case "cpu":
			if bugMultiplier > 0 {
				innerLoops := bugMultiplier * 800
				if innerLoops > 500_000 {
					innerLoops = 500_000
				}
				for j := 0; j < innerLoops; j++ {
					checksum += math.Sin(float64((i+j)%360)) * math.Cos(float64((i*j)%360+1))
				}
			}
		case "memory":
			if bugMultiplier > 0 && i%(bugMultiplier+1) == 0 {
				chunkSize := 64 * 1024 * bugMultiplier
				if chunkSize > 8*1024*1024 {
					chunkSize = 8 * 1024 * 1024
				}
				chunk := make([]byte, chunkSize)
				for k := range chunk {
					chunk[k] = byte((i + k) % 255)
				}
				allocated += len(chunk)
				appendLeak(chunk, leakCap)
			}
		case "mixed":
			if bugMultiplier > 0 {
				innerLoops := bugMultiplier * 400
				if innerLoops > 250_000 {
					innerLoops = 250_000
				}
				for j := 0; j < innerLoops; j++ {
					checksum += math.Sin(float64(j*i + j))
				}
				if i%(bugMultiplier+1) == 0 {
					chunkSize := 32 * 1024 * bugMultiplier
					if chunkSize > 4*1024*1024 {
						chunkSize = 4 * 1024 * 1024
					}
					chunk := make([]byte, chunkSize)
					allocated += len(chunk)
					appendLeak(chunk, leakCap)
				}
			}
		}
	}
	checksum = math.Mod(checksum, 1e9+7)
	return checksum, allocated, currentRetainedBytes()
}

func appendLeak(chunk []byte, capBytes int) {
	leakMu.Lock()
	defer leakMu.Unlock()

	leakChunks = append(leakChunks, chunk)
	retainedBytes += len(chunk)

	if capBytes <= 0 {
		return
	}

	for retainedBytes > capBytes && len(leakChunks) > 0 {
		removed := len(leakChunks[0])
		leakChunks = leakChunks[1:]
		retainedBytes -= removed
	}
}

func currentRetainedBytes() int {
	leakMu.Lock()
	defer leakMu.Unlock()
	return retainedBytes
}

func setLastStats(stats WorkStats) {
	statsMu.Lock()
	defer statsMu.Unlock()
	lastStats = stats
}

func getLastStats() WorkStats {
	statsMu.RLock()
	defer statsMu.RUnlock()
	return lastStats
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("pprof demo: visit /work to trigger load, /stats for last run, /config for runtime options, /debug/pprof/ for profiling endpoints.\n"))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleWork(w http.ResponseWriter, r *http.Request) {
	stats := runWorkOnce()
	setLastStats(stats)
	writeJSON(w, stats)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	stats := getLastStats()
	writeJSON(w, stats)
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, cfg)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func loadConfig() AppConfig {
	cfg := AppConfig{
		ListenAddr:       getEnv("LISTEN_ADDR", ":8080"),
		WorkIterations:   envInt("WORK_ITERATIONS", 1000),
		WorkSleep:        envDuration("WORK_SLEEP_MS", 500*time.Millisecond),
		BugMode:          strings.ToLower(getEnv("BUG_MODE", "none")),
		BugMultiplier:    envInt("BUG_MULTIPLIER", 0),
		LeakCapBytes:     envInt("LEAK_CAP_BYTES", 64*1024*1024),
		BackgroundWorker: envBool("BACKGROUND_WORKER", true),
	}

	if cfg.WorkIterations <= 0 {
		cfg.WorkIterations = 100
	}
	if cfg.WorkIterations > 2_000_000 {
		cfg.WorkIterations = 2_000_000
	}
	if cfg.BugMultiplier < 0 {
		cfg.BugMultiplier = 0
	}
	if cfg.BugMultiplier > 1000 {
		cfg.BugMultiplier = 1000
	}
	if cfg.WorkSleep <= 0 {
		cfg.WorkSleep = 200 * time.Millisecond
	}
	switch cfg.BugMode {
	case "cpu", "memory", "mixed", "none":
	default:
		cfg.BugMode = "none"
	}
	if cfg.LeakCapBytes < 0 {
		cfg.LeakCapBytes = 0
	}
	return cfg
}

func envInt(key string, defaultVal int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("Invalid value %q for %s, using default %d", raw, key, defaultVal)
		return defaultVal
	}
	return val
}

func envDuration(key string, defaultVal time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("Invalid value %q for %s, using default %s", raw, key, defaultVal)
		return defaultVal
	}
	if val <= 0 {
		return defaultVal
	}
	return time.Duration(val) * time.Millisecond
}

func envBool(key string, defaultVal bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("Invalid value %q for %s, using default %t", raw, key, defaultVal)
		return defaultVal
	}
	return val
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func init() {
	runtime.SetBlockProfileRate(100)
	runtime.SetMutexProfileFraction(25)
}
