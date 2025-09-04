package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents log severity.
type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

func ParseLevel(s string) Level {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "debug":
		return Debug
	case "warn":
		return Warn
	case "error":
		return Error
	default:
		return Info
	}
}

type Logger struct {
	min      Level
	json     bool
	out      io.Writer
	mu       sync.Mutex
	throttle map[string]throttleEntry
}

func New(level string, jsonOut bool) *Logger {
	out := io.Writer(os.Stderr)
	if jsonOut {
		out = os.Stdout
	}
	return &Logger{min: ParseLevel(level), json: jsonOut, out: out}
}

func (l *Logger) Enabled(v Level) bool { return v >= l.min }

func (l *Logger) Debugf(format string, a ...any) { l.log(Debug, fmt.Sprintf(format, a...)) }
func (l *Logger) Infof(format string, a ...any)  { l.log(Info, fmt.Sprintf(format, a...)) }
func (l *Logger) Warnf(format string, a ...any)  { l.log(Warn, fmt.Sprintf(format, a...)) }
func (l *Logger) Errorf(format string, a ...any) { l.log(Error, fmt.Sprintf(format, a...)) }

func (l *Logger) log(level Level, msg string) {
	if !l.Enabled(level) {
		return
	}
	lvl := levelString(level)
	if l.json {
		payload := map[string]any{
			"ts":    time.Now().Format(time.RFC3339Nano),
			"level": lvl,
			"msg":   msg,
		}
		_ = json.NewEncoder(l.out).Encode(payload)
		return
	}
	_, _ = fmt.Fprintf(l.out, "%s\t%s\n", strings.ToUpper(lvl), msg)
}

func levelString(l Level) string {
	switch l {
	case Debug:
		return "debug"
	case Warn:
		return "warn"
	case Error:
		return "error"
	default:
		return "info"
	}
}

// throttleEntry tracks throttling window and suppressed count for a key.
type throttleEntry struct {
	until      time.Time
	suppressed int
}

// WarnfThrottled logs a warning at most once per window for a given key.
// Repeated calls within the window are suppressed and summarized on the next log.
func (l *Logger) WarnfThrottled(key string, window time.Duration, format string, a ...any) {
	if l == nil {
		return
	}
	if window <= 0 {
		window = time.Second
	}
	l.mu.Lock()
	if l.throttle == nil {
		l.throttle = make(map[string]throttleEntry)
	}
	e := l.throttle[key]
	now := time.Now()
	if now.Before(e.until) {
		e.suppressed++
		l.throttle[key] = e
		l.mu.Unlock()
		return
	}
	supp := e.suppressed
	e.until = now.Add(window)
	e.suppressed = 0
	l.throttle[key] = e
	l.mu.Unlock()
	msg := fmt.Sprintf(format, a...)
	if supp > 0 {
		msg = fmt.Sprintf("%s (suppressed %d similar warnings)", msg, supp)
	}
	l.log(Warn, msg)
}
