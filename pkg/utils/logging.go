package utils

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// LogEntry represents a parsed log line.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Module    string `json:"module"`
	Message   string `json:"message"`
}

var (
	logFile     *os.File
	logBuf      *bufio.Writer
	logMu       sync.Mutex // guards logFile + logBuf during rotation
	logFilePath string
	logFileOnce sync.Once

	subscribersMu sync.Mutex
	subscribers   []chan LogEntry

	logLineRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) \[(\w+)\] \[([^\]]+)\] (.*)$`)
)

// Subscribe creates a buffered channel that receives real-time log entries.
func Subscribe() chan LogEntry {
	ch := make(chan LogEntry, 256)
	subscribersMu.Lock()
	subscribers = append(subscribers, ch)
	subscribersMu.Unlock()
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func Unsubscribe(ch chan LogEntry) {
	subscribersMu.Lock()
	defer subscribersMu.Unlock()
	for i, s := range subscribers {
		if s == ch {
			subscribers = append(subscribers[:i], subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// emit sends a log entry to all subscribers (non-blocking).
func emit(entry LogEntry) {
	subscribersMu.Lock()
	subs := make([]chan LogEntry, len(subscribers))
	copy(subs, subscribers)
	subscribersMu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- entry:
		default:
			// drop if subscriber is slow
		}
	}
}

// ParseLogLine parses a log line into a LogEntry.
// Lines that don't match the format are preserved as Level="RAW".
func ParseLogLine(line string) (LogEntry, bool) {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return LogEntry{}, false
	}
	m := logLineRe.FindStringSubmatch(line)
	if m == nil {
		return LogEntry{
			Timestamp: "",
			Level:     "RAW",
			Module:    "",
			Message:   line,
		}, true
	}
	return LogEntry{
		Timestamp: m[1],
		Level:     m[2],
		Module:    m[3],
		Message:   m[4],
	}, true
}

// FlushLog flushes the buffered log writer to disk.
func FlushLog() {
	logMu.Lock()
	defer logMu.Unlock()
	if logBuf != nil {
		logBuf.Flush()
	}
}

// rotateLogLoop runs daily at 00:10 to archive the current log file.
func rotateLogLoop() {
	for {
		now := time.Now()
		// Next rotation at 00:10 today or tomorrow.
		next := time.Date(now.Year(), now.Month(), now.Day(), 0, 10, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(next.Sub(now))
		rotateLog()
	}
}

// rotateLog archives the current log file as arb_YYYYMMDD.log (yesterday's date)
// and opens a fresh log file.
func rotateLog() {
	logMu.Lock()
	defer logMu.Unlock()

	if logFile == nil {
		return
	}

	// Flush and close current file.
	logBuf.Flush()
	logFile.Close()

	// Archive: arb.log → arb_20260321.log (yesterday)
	yesterday := time.Now().AddDate(0, 0, -1).Format("20060102")
	dir := filepath.Dir(logFilePath)
	ext := filepath.Ext(logFilePath)
	base := strings.TrimSuffix(filepath.Base(logFilePath), ext)
	archivePath := filepath.Join(dir, base+"_"+yesterday+ext)
	os.Rename(logFilePath, archivePath)

	// Open new log file.
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log rotation: failed to open new log file: %v\n", err)
		// Try to reopen the archive as fallback.
		os.Rename(archivePath, logFilePath)
		f, _ = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}
	logFile = f
	logBuf = bufio.NewWriterSize(f, 64*1024)
}

// GetLogFilePath returns the resolved log file path.
func GetLogFilePath() string {
	getLogWriter() // ensure initialized
	return logFilePath
}

// TailLogFile reads the last N log entries from the log file using reverse seek.
func TailLogFile(limit int) []LogEntry {
	path := GetLogFilePath()
	if path == "" {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return nil
	}

	// Read chunks from the end of file
	const chunkSize = 8192
	fileSize := info.Size()
	var lines []string
	remaining := fileSize

	for remaining > 0 && len(lines) <= limit {
		readSize := int64(chunkSize)
		if readSize > remaining {
			readSize = remaining
		}
		offset := remaining - readSize
		buf := make([]byte, readSize)
		n, err := f.ReadAt(buf, offset)
		if err != nil && err != io.EOF {
			break
		}
		chunk := string(buf[:n])

		// Split into lines and prepend to collected lines
		parts := strings.Split(chunk, "\n")
		if len(lines) > 0 {
			// Join last part of this chunk with first part of previous chunk
			parts[len(parts)-1] += lines[0]
			lines = lines[1:]
		}
		lines = append(parts, lines...)
		remaining = offset
	}

	// Parse lines from the end, collecting up to limit entries
	var entries []LogEntry
	for i := len(lines) - 1; i >= 0 && len(entries) < limit; i-- {
		entry, ok := ParseLogLine(lines[i])
		if ok {
			entries = append(entries, entry)
		}
	}

	// Reverse to chronological order
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries
}

// logWriter is a thread-safe writer that always writes to the current logBuf,
// surviving log rotations. It implements io.Writer.
type logWriter struct{}

func (w *logWriter) Write(p []byte) (n int, err error) {
	// Write to stdout unconditionally.
	os.Stdout.Write(p)
	// Write to log file under lock (logBuf may change during rotation).
	logMu.Lock()
	if logBuf != nil {
		n, err = logBuf.Write(p)
	}
	logMu.Unlock()
	return len(p), nil
}

var sharedLogWriter = &logWriter{}

func getLogWriter() io.Writer {
	logFileOnce.Do(func() {
		path := os.Getenv("LOG_FILE")
		if path == "" {
			path = "logs/arb.log"
		}
		// Ensure parent directory exists.
		if dir := filepath.Dir(path); dir != "." {
			os.MkdirAll(dir, 0755)
		}
		logFilePath = path
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file %s: %v\n", path, err)
			return
		}
		logFile = f
		logBuf = bufio.NewWriterSize(f, 64*1024) // 64KB buffer
		// Flush every 2 seconds so logs aren't delayed too long.
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			for range ticker.C {
				logMu.Lock()
				if logBuf != nil {
					logBuf.Flush()
				}
				logMu.Unlock()
			}
		}()
		// Log rotation: at 00:10 each day, archive arb.log → arb_YYYYMMDD.log (yesterday's date).
		go rotateLogLoop()
	})
	return sharedLogWriter
}

// Logger provides structured logging with module context.
type Logger struct {
	module string
	logger *log.Logger
}

// NewLogger creates a logger for a specific module.
func NewLogger(module string) *Logger {
	return &Logger{
		module: module,
		logger: log.New(getLogWriter(), "", 0),
	}
}

func (l *Logger) format(level, msg string, args ...interface{}) string {
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	formatted := fmt.Sprintf(msg, args...)
	return fmt.Sprintf("%s [%s] [%s] %s", ts, level, l.module, formatted)
}

func (l *Logger) Info(msg string, args ...interface{}) {
	line := l.format("INFO", msg, args...)
	l.logger.Println(line)
	emit(LogEntry{
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		Level:     "INFO",
		Module:    l.module,
		Message:   fmt.Sprintf(msg, args...),
	})
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	line := l.format("WARN", msg, args...)
	l.logger.Println(line)
	emit(LogEntry{
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		Level:     "WARN",
		Module:    l.module,
		Message:   fmt.Sprintf(msg, args...),
	})
}

func (l *Logger) Error(msg string, args ...interface{}) {
	line := l.format("ERROR", msg, args...)
	l.logger.Println(line)
	emit(LogEntry{
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
		Level:     "ERROR",
		Module:    l.module,
		Message:   fmt.Sprintf(msg, args...),
	})
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		line := l.format("DEBUG", msg, args...)
		l.logger.Println(line)
		emit(LogEntry{
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000"),
			Level:     "DEBUG",
			Module:    l.module,
			Message:   fmt.Sprintf(msg, args...),
		})
	}
}
