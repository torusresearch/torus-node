package logging

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LevelMapping maps LogLevels into expected keys
var LevelMapping = map[LogLevel]string{
	DEBUG:   "DEBUG",
	INFO:    "INFO",
	WARNING: "WARNING",
	ERROR:   "ERROR",
	FATAL:   "FATAL",
}

type LogLevel int

// LogLevel mappings
const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
	FATAL
)

// LogMessage encapsulates all the data for one line of log message
type LogMessage struct {
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

func (l *LogMessage) Bytes() []byte {
	// TODO: Add formatter etc.
	return []byte(fmt.Sprintf("[%s] %s - %s\n", l.Severity, time.Now().Format(time.RFC3339), l.Message))
}

// Logger is an interface that represents minimal expected functionality of a leveled logger.
type Logger interface {
	Debug(string)
	Info(string)
	Warning(string)
	Error(string)
	Fatal(string)
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Warningf(string, ...interface{})
	Errorf(string, ...interface{})
	Fatalf(string, ...interface{})

	WithLevel(LogLevel) Logger
	WithLevelString(string) Logger
}

// Logger is a struct containing logger data
type logger struct {
	Out io.Writer

	mu    sync.Mutex
	Level LogLevel // Only log messages with LogLevel > Level stored here
}

// TODO: We should optimize around memory allocations when that becomes a problem
func (l *logger) write(message string, severity LogLevel) {
	if severity < l.Level {
		return
	}

	logM := LogMessage{
		Message:  message,
		Severity: LevelMapping[severity],
	}

	l.Out.Write(logM.Bytes())
}

// Debug writes to Out with level Debug
func (l *logger) Debug(message string) {
	l.write(message, DEBUG)
}

// Info writes to Out with level Info
func (l *logger) Info(message string) {
	l.write(message, INFO)
}

// Warning writes to Out with level Warning
func (l *logger) Warning(message string) {
	l.write(message, WARNING)
}

// Error writes to Out with level Error
func (l *logger) Error(message string) {
	l.write(message, ERROR)
}

// Fatal writes to Out with level Fatal
func (l *logger) Fatal(message string) {
	l.write(message, FATAL)
	panic(message)
}

// Debugf formats according to a format specifier and writes to Out with level Debug
func (l *logger) Debugf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	l.write(message, DEBUG)
}

// Infof formats according to a format specifier and writes to Out with level Info
func (l *logger) Infof(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	l.write(message, INFO)
}

// Warningf formats according to a format specifier and writes to Out with level Warning
func (l *logger) Warningf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	l.write(message, WARNING)
}

// Errorf formats according to a format specifier and writes to Out with level Error
func (l *logger) Errorf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	l.write(message, ERROR)
}

// Fatalf formats according to a format specifier and writes to Out with level Fatal which causes the program to panic
func (l *logger) Fatalf(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	l.write(message, FATAL)
	panic(message)
}

func (l *logger) With(keyvals ...interface{}) Logger {
	return l
}

// NewDefault returns a default logger
func NewDefault() Logger {
	return (&logger{
		Out: os.Stdout,
	}).WithLevel(INFO)

}

// SetLevel sets the log level of the package logger
func SetLevel(level LogLevel) {
	l, ok := std.(*logger)
	if ok {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.Level = level
	}
}

func getLevelFromString(levelString string) LogLevel {
	var level LogLevel
	switch levelString {
	case "debug":
		level = DEBUG
	case "info":
		level = INFO
	case "warn":
		level = WARNING
	case "error":
		level = ERROR
	case "fatal":
		level = FATAL
	default:
		std.Info("logging - unknown LogLevel, using default level: INFO")
		level = INFO
	}
	return level

}

// SetLevel sets the log level of the package logger
func SetLevelString(levelString string) {
	level := getLevelFromString(levelString)
	l, ok := std.(*logger)
	if ok {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.Level = level
	}
}

func (l *logger) WithLevel(level LogLevel) Logger {
	l.Level = level
	return l
}

func (l *logger) WithLevelString(levelString string) Logger {
	l.Level = getLevelFromString(levelString)
	return l
}

// This a default logger that is exported on a package level.

var std = NewDefault()

// Debug writes to Out with level Debug
func Debug(msg string) {
	std.Debug(msg)
}

// Info writes to Out with level Info
func Info(msg string) {
	std.Info(msg)
}

// Warning writes to Out with level Warning
func Warning(msg string) {
	std.Warning(msg)
}

// Error writes to Out with level Error
func Error(msg string) {
	std.Error(msg)
}

// Fatal writes to Out with level Fatal
func Fatal(msg string) {
	std.Fatal(msg)
}

// Debugf formats according to a format specifier and writes to Out with level Debug
func Debugf(format string, a ...interface{}) {
	std.Debugf(format, a...)
}

// Infof formats according to a format specifier and writes to Out with level Info
func Infof(format string, a ...interface{}) {
	std.Infof(format, a...)
}

// Warningf formats according to a format specifier and writes to Out with level Warning
func Warningf(format string, a ...interface{}) {
	std.Warningf(format, a...)
}

// Errorf formats according to a format specifier and writes to Out with level Error
func Errorf(format string, a ...interface{}) {
	std.Errorf(format, a...)
}

// Fatalf formats according to a format specifier and writes to Out with level Fatal
func Fatalf(format string, a ...interface{}) {
	std.Fatalf(format, a...)
}
