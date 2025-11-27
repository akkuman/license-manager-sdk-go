package logger

import (
	"log"
	"os"
)

// Logger is a minimal log interface used across modules.
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// StdLogger is the default implementation backed by log.Logger.
type StdLogger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	err   *log.Logger
	level Level
}

// Level controls log verbosity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// ParseLevel converts string to Level, defaults to info.
func ParseLevel(lvl string) Level {
	switch lvl {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// NewStdLogger returns a StdLogger that writes to stdout/stderr.
func NewStdLogger(level Level) *StdLogger {
	flags := log.LstdFlags | log.Lmicroseconds
	return &StdLogger{
		debug: log.New(os.Stdout, "[DEBUG] ", flags),
		info:  log.New(os.Stdout, "[INFO ] ", flags),
		warn:  log.New(os.Stdout, "[WARN ] ", flags),
		err:   log.New(os.Stderr, "[ERROR] ", flags),
		level: level,
	}
}

func (l *StdLogger) Debugf(format string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.debug.Printf(format, args...)
	}
}

func (l *StdLogger) Infof(format string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.info.Printf(format, args...)
	}
}

func (l *StdLogger) Warnf(format string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.warn.Printf(format, args...)
	}
}

func (l *StdLogger) Errorf(format string, args ...interface{}) {
	if l.level <= LevelError {
		l.err.Printf(format, args...)
	}
}
