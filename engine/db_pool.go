package engine

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// DBPool gestiona un pool de conexiones optimizado
type DBPool struct {
	db          *sql.DB
	maxConns    int
	maxIdleTime time.Duration
	stats       *DBStats
}

// DBStats almacena estadísticas del pool
type DBStats struct {
	TotalQueries  int64
	ActiveConns   int
	IdleConns     int
	WaitTime      time.Duration
	LastError     error
	LastErrorTime time.Time
	mu            sync.RWMutex
}

var (
	dbPool     *DBPool
	dbPoolOnce sync.Once
)

// InitDBPool inicializa el pool de conexiones
func InitDBPool() error {
	var initErr error

	dbPoolOnce.Do(func() {
		config := GetConfig()
		db, err := sql.Open(config.DatabaseNflow.Driver, config.DatabaseNflow.DSN)
		if err != nil {
			initErr = fmt.Errorf("error abriendo base de datos: %v", err)
			return
		}

		// Configurar pool
		maxOpenConns := 25
		maxIdleConns := 5
		connMaxLifetime := 5 * time.Minute
		connMaxIdleTime := 1 * time.Minute

		// Ajustar según el driver
		switch config.DatabaseNflow.Driver {
		case "sqlite3":
			maxOpenConns = 1 // SQLite funciona mejor con una sola conexión
			maxIdleConns = 1
		case "postgres":
			maxOpenConns = 50
			maxIdleConns = 10
		case "mysql":
			maxOpenConns = 40
			maxIdleConns = 10
		}

		db.SetMaxOpenConns(maxOpenConns)
		db.SetMaxIdleConns(maxIdleConns)
		db.SetConnMaxLifetime(connMaxLifetime)
		db.SetConnMaxIdleTime(connMaxIdleTime)

		// Verificar conexión
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			initErr = fmt.Errorf("error verificando conexión: %v", err)
			db.Close()
			return
		}

		dbPool = &DBPool{
			db:          db,
			maxConns:    maxOpenConns,
			maxIdleTime: connMaxIdleTime,
			stats:       &DBStats{},
		}

		// Iniciar monitor de estadísticas
		go dbPool.monitorStats()

		log.Printf("Pool de base de datos inicializado: driver=%s, maxConns=%d",
			config.DatabaseNflow.Driver, maxOpenConns)
	})

	return initErr
}

// GetDBOptimized obtiene una conexión del pool optimizado
func GetDBOptimized() (*sql.DB, error) {
	if err := InitDBPool(); err != nil {
		return nil, err
	}

	if dbPool == nil || dbPool.db == nil {
		return nil, fmt.Errorf("pool de base de datos no inicializado")
	}

	// Actualizar estadísticas
	dbPool.stats.mu.Lock()
	dbPool.stats.TotalQueries++
	dbPool.stats.mu.Unlock()

	return dbPool.db, nil
}

// ExecuteWithRetry ejecuta una query con reintentos
func ExecuteWithRetry(ctx context.Context, fn func(*sql.Conn) error) error {
	db, err := GetDBOptimized()
	if err != nil {
		return err
	}

	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}

		err = fn(conn)
		conn.Close()

		if err == nil {
			return nil
		}

		// Si es un error de contexto, no reintentar
		if ctx.Err() != nil {
			return ctx.Err()
		}

		lastErr = err
		log.Printf("Error en intento %d/%d: %v", i+1, maxRetries, err)

		// Esperar antes de reintentar
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}

	// Registrar error en estadísticas
	dbPool.stats.mu.Lock()
	dbPool.stats.LastError = lastErr
	dbPool.stats.LastErrorTime = time.Now()
	dbPool.stats.mu.Unlock()

	return fmt.Errorf("fallo después de %d intentos: %v", maxRetries, lastErr)
}

// QueryWithTimeout ejecuta una query con timeout
func QueryWithTimeout(ctx context.Context, timeout time.Duration, query string, args ...interface{}) (*sql.Rows, error) {
	db, err := GetDBOptimized()
	if err != nil {
		return nil, err
	}

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return db.QueryContext(ctx, query, args...)
}

// PreparedStatement gestiona statements preparados de forma segura
type PreparedStatement struct {
	stmt    *sql.Stmt
	query   string
	mu      sync.Mutex
	uses    int64
	lastUse time.Time
}

var (
	preparedStmts = make(map[string]*PreparedStatement)
	preparedMu    sync.RWMutex
)

// GetPreparedStatement obtiene o crea un statement preparado
func GetPreparedStatement(query string) (*PreparedStatement, error) {
	preparedMu.RLock()
	if ps, ok := preparedStmts[query]; ok {
		preparedMu.RUnlock()
		ps.mu.Lock()
		ps.uses++
		ps.lastUse = time.Now()
		ps.mu.Unlock()
		return ps, nil
	}
	preparedMu.RUnlock()

	// Crear nuevo statement
	preparedMu.Lock()
	defer preparedMu.Unlock()

	// Verificar de nuevo por si otro goroutine lo creó
	if ps, ok := preparedStmts[query]; ok {
		return ps, nil
	}

	db, err := GetDBOptimized()
	if err != nil {
		return nil, err
	}

	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, err
	}

	ps := &PreparedStatement{
		stmt:    stmt,
		query:   query,
		lastUse: time.Now(),
	}

	preparedStmts[query] = ps

	return ps, nil
}

// monitorStats monitorea las estadísticas del pool
func (p *DBPool) monitorStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := p.db.Stats()

		p.stats.mu.Lock()
		p.stats.ActiveConns = stats.OpenConnections
		p.stats.IdleConns = stats.Idle
		p.stats.WaitTime = stats.WaitDuration
		p.stats.mu.Unlock()

		// Log si hay muchas conexiones esperando
		if stats.WaitCount > 10 {
			log.Printf("Advertencia: %d conexiones esperando, considere aumentar el pool", stats.WaitCount)
		}
	}
}

// GetDBStats retorna las estadísticas actuales del pool
func GetDBStats() map[string]interface{} {
	if dbPool == nil || dbPool.stats == nil {
		return map[string]interface{}{
			"status": "no inicializado",
		}
	}

	dbPool.stats.mu.RLock()
	defer dbPool.stats.mu.RUnlock()

	dbStats := dbPool.db.Stats()

	return map[string]interface{}{
		"total_queries":   dbPool.stats.TotalQueries,
		"active_conns":    dbPool.stats.ActiveConns,
		"idle_conns":      dbPool.stats.IdleConns,
		"wait_time_ms":    dbPool.stats.WaitTime.Milliseconds(),
		"max_open_conns":  dbStats.MaxOpenConnections,
		"in_use":          dbStats.InUse,
		"wait_count":      dbStats.WaitCount,
		"last_error":      fmt.Sprint(dbPool.stats.LastError),
		"last_error_time": dbPool.stats.LastErrorTime.Format(time.RFC3339),
	}
}

// CloseDBPool cierra el pool de conexiones
func CloseDBPool() error {
	if dbPool != nil && dbPool.db != nil {
		// Cerrar statements preparados
		preparedMu.Lock()
		for _, ps := range preparedStmts {
			ps.stmt.Close()
		}
		preparedStmts = make(map[string]*PreparedStatement)
		preparedMu.Unlock()

		// Cerrar pool
		err := dbPool.db.Close()
		dbPool = nil
		return err
	}
	return nil
}
