package engine

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arturoeanton/nflow-runtime/logger"
)

type TrackerEntry struct {
	LogId, BoxId, BoxName, BoxType        string
	Username, IP, RealIP, URL             string
	ConnectionNext                        string
	Diff                                  time.Duration
	OrderBox                              int
	JSONPayload                           []byte
	UserAgent, QueryParam, Hostname, Host string
}

type TrackerStats struct {
	Processed   int64
	Errors      int64
	Dropped     int64
	BatchCount  int64
	LastProcess time.Time
}

type BatchProcessor struct {
	batch     []TrackerEntry
	batchSize int
	mutex     sync.Mutex
	ticker    *time.Ticker
	db        *sql.DB
	config    *ConfigWorkspace
}

var (
	trackerChannel       chan TrackerEntry
	trackerStats         = &TrackerStats{}
	trackerEnabled       = int32(0) // Default disabled, will be set from config
	batchProcessors      []*BatchProcessor
	shutdownChan         = make(chan struct{})
	shutdownOnce         sync.Once
	maxRetries           = 3
	retryDelay           = 100 * time.Millisecond
	circuitBreaker       = int32(0) // 0 = closed, 1 = open
	consecutiveErrors    = int64(0)
	maxConsecutiveErrors = int64(50)
	trackerConfig        *TrackerConfig
)

// StartTracker initializes the high-performance tracker system
func StartTracker(numWorkers int) {
	// Get config and check if tracker is enabled
	config := GetConfigReference()
	trackerConfig = &config.TrackerConfig

	// If tracker is disabled in config, don't start it
	if !trackerConfig.Enabled {
		logger.Info("Tracker is disabled in configuration")
		return
	}

	// Set enabled flag based on config
	if trackerConfig.Enabled {
		atomic.StoreInt32(&trackerEnabled, 1)
	}

	// Use config values with fallbacks
	if trackerConfig.Workers > 0 {
		numWorkers = trackerConfig.Workers
	}

	bufferSize := 100000
	if trackerConfig.ChannelBuffer > 0 {
		bufferSize = trackerConfig.ChannelBuffer
	}

	// Initialize channel with configured buffer size
	trackerChannel = make(chan TrackerEntry, bufferSize)

	// Only log if verbose logging is enabled
	if trackerConfig.VerboseLogging {
		logger.Info("Starting optimized tracker with", numWorkers, "workers")
	}

	// Initialize batch processors
	batchProcessors = make([]*BatchProcessor, numWorkers)

	// Create a single DB connection pool
	db, err := GetDB()
	if err != nil {
		logger.Error("Failed to get DB for tracker:", err)
		return
	}

	batchSize := 100
	if trackerConfig.BatchSize > 0 {
		batchSize = trackerConfig.BatchSize
	}

	flushInterval := 250
	if trackerConfig.FlushInterval > 0 {
		flushInterval = trackerConfig.FlushInterval
	}

	for i := 0; i < numWorkers; i++ {
		bp := &BatchProcessor{
			batch:     make([]TrackerEntry, 0, batchSize),
			batchSize: batchSize,
			ticker:    time.NewTicker(time.Duration(flushInterval) * time.Millisecond),
			db:        db,
			config:    config,
		}

		batchProcessors[i] = bp
		go bp.start(i)
	}

	// Start circuit breaker monitor
	go monitorCircuitBreaker()

	// Start stats reporter only if verbose logging is enabled
	if trackerConfig.VerboseLogging {
		go reportStats()
	}
}

// BatchProcessor methods for high-performance batch processing
func (bp *BatchProcessor) start(workerID int) {
	if trackerConfig != nil && trackerConfig.VerboseLogging {
		logger.Verbose("Starting batch processor worker", workerID)
	}
	defer bp.ticker.Stop()

	for {
		select {
		case entry := <-trackerChannel:
			// Quick circuit breaker check
			if atomic.LoadInt32(&circuitBreaker) == 1 {
				atomic.AddInt64(&trackerStats.Dropped, 1)
				continue
			}

			// Check if tracker is enabled
			if atomic.LoadInt32(&trackerEnabled) == 0 {
				atomic.AddInt64(&trackerStats.Dropped, 1)
				continue
			}

			bp.addToBatch(entry)

		case <-bp.ticker.C:
			// Periodic flush
			bp.flushBatch()

		case <-shutdownChan:
			// Graceful shutdown - flush remaining entries
			bp.flushBatch()
			if trackerConfig != nil && trackerConfig.VerboseLogging {
				logger.Info("Tracker worker", workerID, "shutdown complete")
			}
			return
		}
	}
}

func (bp *BatchProcessor) addToBatch(entry TrackerEntry) {
	bp.mutex.Lock()
	bp.batch = append(bp.batch, entry)
	batchLen := len(bp.batch)
	bp.mutex.Unlock()

	// Flush if batch is full
	if batchLen >= bp.batchSize {
		bp.flushBatch()
	}
}

func (bp *BatchProcessor) flushBatch() {
	bp.mutex.Lock()
	if len(bp.batch) == 0 {
		bp.mutex.Unlock()
		return
	}

	// Copy batch and reset
	batchToProcess := make([]TrackerEntry, len(bp.batch))
	copy(batchToProcess, bp.batch)
	bp.batch = bp.batch[:0] // Reset slice but keep capacity
	bp.mutex.Unlock()

	// Process batch without blocking
	go bp.processBatch(batchToProcess)
}

func (bp *BatchProcessor) processBatch(batch []TrackerEntry) {
	if bp.config.DatabaseNflow.QueryInsertLog == "" {
		return // Skip if no query configured
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var err error
	for retry := 0; retry < maxRetries; retry++ {
		err = bp.insertBatch(ctx, batch)
		if err == nil {
			break
		}

		// Exponential backoff
		time.Sleep(retryDelay * time.Duration(1<<retry))
	}

	if err != nil {
		atomic.AddInt64(&trackerStats.Errors, 1)
		atomic.AddInt64(&consecutiveErrors, 1)
		if trackerConfig != nil && trackerConfig.VerboseLogging {
			logger.Error("Failed to process tracker batch after retries:", err)
		}
	} else {
		atomic.StoreInt64(&consecutiveErrors, 0)
		atomic.AddInt64(&trackerStats.Processed, int64(len(batch)))
		atomic.AddInt64(&trackerStats.BatchCount, 1)
		trackerStats.LastProcess = time.Now()
	}
}

func (bp *BatchProcessor) insertBatch(ctx context.Context, batch []TrackerEntry) error {
	if len(batch) == 0 {
		return nil
	}

	// Use transaction for batch insert
	tx, err := bp.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement once for the batch
	stmt, err := tx.PrepareContext(ctx, bp.config.DatabaseNflow.QueryInsertLog)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Pre-allocate string buffer for better performance
	var diffStr string

	// Execute all entries in the batch
	for _, entry := range batch {
		// Format time more efficiently
		diffStr = fmt.Sprintf("%dm", entry.Diff.Milliseconds())

		_, err = stmt.ExecContext(ctx,
			entry.LogId,
			entry.BoxId,
			entry.BoxName,
			entry.BoxType,
			entry.URL,
			entry.Username,
			entry.ConnectionNext,
			diffStr,
			entry.OrderBox,
			entry.JSONPayload, // Already []byte, no need to convert to string
			entry.IP,
			entry.RealIP,
			entry.UserAgent,
			entry.QueryParam,
			entry.Hostname,
			entry.Host,
		)
		if err != nil {
			return fmt.Errorf("failed to execute statement: %w", err)
		}
	}

	return tx.Commit()
}

// Circuit breaker and monitoring functions
func monitorCircuitBreaker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			errors := atomic.LoadInt64(&consecutiveErrors)
			if errors >= maxConsecutiveErrors {
				atomic.StoreInt32(&circuitBreaker, 1)
				if trackerConfig != nil && trackerConfig.VerboseLogging {
					logger.Error("Tracker circuit breaker OPEN - too many consecutive errors:", errors)
				}

				// Auto-recovery after 30 seconds
				time.Sleep(30 * time.Second)
				atomic.StoreInt32(&circuitBreaker, 0)
				atomic.StoreInt64(&consecutiveErrors, 0)
				if trackerConfig != nil && trackerConfig.VerboseLogging {
					logger.Info("Tracker circuit breaker CLOSED - attempting recovery")
				}
			}
		case <-shutdownChan:
			return
		}
	}
}

func reportStats() {
	interval := 300 // default 5 minutes
	if trackerConfig != nil && trackerConfig.StatsInterval > 0 {
		interval = trackerConfig.StatsInterval
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			processed := atomic.LoadInt64(&trackerStats.Processed)
			errors := atomic.LoadInt64(&trackerStats.Errors)
			dropped := atomic.LoadInt64(&trackerStats.Dropped)
			batches := atomic.LoadInt64(&trackerStats.BatchCount)

			if (processed > 0 || errors > 0 || dropped > 0) && trackerConfig != nil && trackerConfig.VerboseLogging {
				logger.Info(fmt.Sprintf("Tracker Stats - Processed: %d, Errors: %d, Dropped: %d, Batches: %d, Channel: %d/%d",
					processed, errors, dropped, batches, len(trackerChannel), cap(trackerChannel)))
			}
		case <-shutdownChan:
			return
		}
	}
}

// Utility functions
func GetConfigReference() *ConfigWorkspace {
	// Return reference to avoid copying large config struct
	config := GetConfig()
	return config
}

// Public API functions for runtime control
func IsTrackerEnabled() bool {
	return atomic.LoadInt32(&trackerEnabled) == 1
}

func DisableTracker() {
	atomic.StoreInt32(&trackerEnabled, 0)
	if trackerConfig != nil && trackerConfig.VerboseLogging {
		logger.Info("Tracker disabled")
	}
}

func EnableTracker() {
	atomic.StoreInt32(&trackerEnabled, 1)
	if trackerConfig != nil && trackerConfig.VerboseLogging {
		logger.Info("Tracker enabled")
	}
}

func GetTrackerStats() TrackerStats {
	return TrackerStats{
		Processed:   atomic.LoadInt64(&trackerStats.Processed),
		Errors:      atomic.LoadInt64(&trackerStats.Errors),
		Dropped:     atomic.LoadInt64(&trackerStats.Dropped),
		BatchCount:  atomic.LoadInt64(&trackerStats.BatchCount),
		LastProcess: trackerStats.LastProcess,
	}
}

func ShutdownTracker() {
	shutdownOnce.Do(func() {
		if trackerConfig != nil && trackerConfig.VerboseLogging {
			logger.Info("Shutting down tracker system...")
		}
		close(shutdownChan)

		// Give workers time to flush remaining entries
		time.Sleep(1 * time.Second)

		// Final stats
		stats := GetTrackerStats()
		if trackerConfig != nil && trackerConfig.VerboseLogging {
			logger.Info(fmt.Sprintf("Tracker shutdown complete - Final stats: Processed: %d, Errors: %d, Dropped: %d",
				stats.Processed, stats.Errors, stats.Dropped))
		}
	})
}
