package telemetry

import (
	"fmt"
	"log"
	"time"
)

// Logger keeps structured logging minimal but consistent.
type Logger struct{}

func NewLogger() *Logger { return &Logger{} }

func (l *Logger) Info(scope string, kv ...any) {
	l.logWithLevel("INFO", scope, kv...)
}

func (l *Logger) Error(scope string, kv ...any) {
	l.logWithLevel("ERROR", scope, kv...)
}

func (l *Logger) logWithLevel(level, scope string, kv ...any) {
	msg := fmt.Sprintf("[%s] [%s] %s", time.Now().Format(time.RFC3339), level, scope)
	if len(kv) > 0 {
		msg = fmt.Sprintf("%s %v", msg, kv)
	}
	log.Println(msg)
}
