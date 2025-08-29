package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
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
	min   Level
	json  bool
	out   io.Writer
}

func New(level string, jsonOut bool) *Logger {
	out := io.Writer(os.Stderr)
	if jsonOut { out = os.Stdout }
	return &Logger{min: ParseLevel(level), json: jsonOut, out: out}
}

func (l *Logger) Enabled(v Level) bool { return v >= l.min }

func (l *Logger) Debugf(format string, a ...any) { l.log(Debug, fmt.Sprintf(format, a...)) }
func (l *Logger) Infof(format string, a ...any)  { l.log(Info, fmt.Sprintf(format, a...)) }
func (l *Logger) Warnf(format string, a ...any)  { l.log(Warn, fmt.Sprintf(format, a...)) }
func (l *Logger) Errorf(format string, a ...any) { l.log(Error, fmt.Sprintf(format, a...)) }

func (l *Logger) log(level Level, msg string) {
	if !l.Enabled(level) { return }
	lvl := levelString(level)
	if l.json {
		payload := map[string]any{
			"ts": time.Now().Format(time.RFC3339Nano),
			"level": lvl,
			"msg": msg,
		}
		_ = json.NewEncoder(l.out).Encode(payload)
		return
	}
	fmt.Fprintf(l.out, "%s\t%s\n", strings.ToUpper(lvl), msg)
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

