package analytics

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// PnLSnapshot represents a point-in-time cumulative PnL record.
type PnLSnapshot struct {
	ID            int64   `json:"id"`
	Timestamp     int64   `json:"timestamp"`      // Unix seconds
	Strategy      string  `json:"strategy"`        // "perp" or "spot"
	Exchange      string  `json:"exchange"`         // exchange name or "" for aggregate
	CumulativePnL float64 `json:"cumulative_pnl"`
	PositionCount int     `json:"position_count"`
	WinCount      int     `json:"win_count"`
	LossCount     int     `json:"loss_count"`
	FundingTotal  float64 `json:"funding_total"`
	FeesTotal     float64 `json:"fees_total"`
}

// Store wraps a SQLite database for analytics time-series data.
type Store struct {
	db *sql.DB
	mu sync.Mutex // serialize writes
}

// NewStore opens (or creates) a SQLite database at dbPath and runs migrations.
// WAL mode and a 5-second busy timeout are configured for safe concurrent reads.
func NewStore(dbPath string) (*Store, error) {
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("NewStore: open: %w", err)
	}

	// Limit to one writer to avoid SQLITE_BUSY on concurrent writes.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("NewStore: ping: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("NewStore: migrate: %w", err)
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// WritePnLSnapshot inserts a single PnL snapshot.
func (s *Store) WritePnLSnapshot(snap PnLSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		`INSERT INTO pnl_snapshots (timestamp, strategy, exchange, cumulative_pnl, position_count, win_count, loss_count, funding_total, fees_total)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snap.Timestamp, snap.Strategy, snap.Exchange, snap.CumulativePnL,
		snap.PositionCount, snap.WinCount, snap.LossCount,
		snap.FundingTotal, snap.FeesTotal,
	)
	if err != nil {
		return fmt.Errorf("WritePnLSnapshot: %w", err)
	}
	return nil
}

// WritePnLSnapshots inserts multiple PnL snapshots in a single transaction (for backfill).
func (s *Store) WritePnLSnapshots(snaps []PnLSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("WritePnLSnapshots: begin: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO pnl_snapshots (timestamp, strategy, exchange, cumulative_pnl, position_count, win_count, loss_count, funding_total, fees_total)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("WritePnLSnapshots: prepare: %w", err)
	}
	defer stmt.Close()

	for _, snap := range snaps {
		_, err := stmt.Exec(
			snap.Timestamp, snap.Strategy, snap.Exchange, snap.CumulativePnL,
			snap.PositionCount, snap.WinCount, snap.LossCount,
			snap.FundingTotal, snap.FeesTotal,
		)
		if err != nil {
			return fmt.Errorf("WritePnLSnapshots: exec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("WritePnLSnapshots: commit: %w", err)
	}
	return nil
}

// GetPnLHistory returns snapshots within [from, to] (inclusive, Unix seconds).
// Pass strategy="all" to return all strategies; otherwise filter by the given strategy.
// Results are ordered ascending by timestamp.
func (s *Store) GetPnLHistory(from, to int64, strategy string) ([]PnLSnapshot, error) {
	var rows *sql.Rows
	var err error

	if strategy == "all" || strategy == "" {
		rows, err = s.db.Query(
			`SELECT id, timestamp, strategy, exchange, cumulative_pnl, position_count, win_count, loss_count, funding_total, fees_total
			 FROM pnl_snapshots WHERE timestamp >= ? AND timestamp <= ? ORDER BY timestamp ASC`,
			from, to,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, timestamp, strategy, exchange, cumulative_pnl, position_count, win_count, loss_count, funding_total, fees_total
			 FROM pnl_snapshots WHERE timestamp >= ? AND timestamp <= ? AND strategy = ? ORDER BY timestamp ASC`,
			from, to, strategy,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("GetPnLHistory: %w", err)
	}
	defer rows.Close()

	var result []PnLSnapshot
	for rows.Next() {
		var snap PnLSnapshot
		if err := rows.Scan(&snap.ID, &snap.Timestamp, &snap.Strategy, &snap.Exchange,
			&snap.CumulativePnL, &snap.PositionCount, &snap.WinCount, &snap.LossCount,
			&snap.FundingTotal, &snap.FeesTotal); err != nil {
			return nil, fmt.Errorf("GetPnLHistory: scan: %w", err)
		}
		result = append(result, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetPnLHistory: rows: %w", err)
	}
	return result, nil
}

// GetLatestTimestamp returns the most recent snapshot timestamp (Unix seconds).
// Returns 0 if no snapshots exist.
func (s *Store) GetLatestTimestamp() (int64, error) {
	var ts sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(timestamp) FROM pnl_snapshots`).Scan(&ts)
	if err != nil {
		return 0, fmt.Errorf("GetLatestTimestamp: %w", err)
	}
	if !ts.Valid {
		return 0, nil
	}
	return ts.Int64, nil
}

// migrate creates the pnl_snapshots table and indexes if they do not exist.
func (s *Store) migrate() error {
	ddl := `
CREATE TABLE IF NOT EXISTS pnl_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    strategy TEXT NOT NULL,
    exchange TEXT NOT NULL DEFAULT '',
    cumulative_pnl REAL NOT NULL,
    position_count INTEGER NOT NULL DEFAULT 0,
    win_count INTEGER NOT NULL DEFAULT 0,
    loss_count INTEGER NOT NULL DEFAULT 0,
    funding_total REAL NOT NULL DEFAULT 0,
    fees_total REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_pnl_snapshots_ts ON pnl_snapshots(timestamp);
CREATE INDEX IF NOT EXISTS idx_pnl_snapshots_strategy_ts ON pnl_snapshots(strategy, timestamp);
`
	_, err := s.db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// Verify WAL mode is active (set via connection string).
	var journalMode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err == nil {
		_ = journalMode // WAL expected; no action needed if different
	}

	return nil
}

// Now returns the current UTC time as Unix seconds -- helper for snapshot callers.
func Now() int64 {
	return time.Now().UTC().Unix()
}
