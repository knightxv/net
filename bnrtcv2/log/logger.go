package log

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/guabee/bnrtc/util"
	log "github.com/sirupsen/logrus"
)

type Entry = log.Entry

type Logger struct {
	log.Logger
	Name           string `json:"name"`
	Level          Level  `json:"level"`
	isDefaultLevel bool
}

type Level string

const (
	// PanicLevel level, highest level of severity. Logs and then calls panic with the
	// message passed to Debug, Info, ...
	PanicLevel Level = "panic"
	// FatalLevel level. Logs and then calls `logger.Exit(1)`. It will exit even if the
	// logging level is set to Panic.
	FatalLevel = "fatal"
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// Commonly used for hooks to send errors to an error tracking service.
	ErrorLevel = "error"
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel = "warn"
	// InfoLevel level. General operational entries about what's going on inside the
	// application.
	InfoLevel = "info"
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel = "debug"
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	TraceLevel = "trace"
)

var (
	defaultLogOut io.Writer            = os.Stdout
	currentLogOut io.Writer            = defaultLogOut
	defaultLevel  Level                = InfoLevel
	loggerMap                          = util.NewSafeMap()
	loggerBuffer  *util.CircularBuffer = nil
)

func getLevel(level Level) log.Level {
	switch level {
	case PanicLevel:
		return log.PanicLevel
	case FatalLevel:
		return log.FatalLevel
	case ErrorLevel:
		return log.ErrorLevel
	case WarnLevel:
		return log.WarnLevel
	case InfoLevel:
		return log.InfoLevel
	case DebugLevel:
		return log.DebugLevel
	case TraceLevel:
		return log.TraceLevel
	}
	return log.DebugLevel
}

var globalLoggerMap = make(map[string]Level)

func isValidLevel(level Level) bool {
	switch level {
	case PanicLevel, FatalLevel, ErrorLevel, WarnLevel, InfoLevel, DebugLevel, TraceLevel:
		return true
	default:
		return false
	}
}

func SetDefaultLevel(level Level) {
	if !isValidLevel(level) {
		return
	}

	defaultLevel = level
	for _, _logger := range loggerMap.Items() {
		logger := _logger.(*Logger)
		if logger.isDefaultLevel {
			logger.SetLogLevel(level)
		}
	}
}

func SetLogLevel(name string, level Level) {
	if !isValidLevel(level) {
		return
	}
	globalLoggerMap[name] = level
	if name == "all" || name == "*" {
		for _, _logger := range loggerMap.Items() {
			logger := _logger.(*Logger)
			logger.SetLogLevel(level)
		}
	} else if strings.HasSuffix(name, "*") {
		prefix := name[:len(name)-1]
		for _, _logger := range loggerMap.Items() {
			logger := _logger.(*Logger)
			if strings.HasPrefix(logger.Name, prefix) {
				logger.SetLogLevel(level)
			}
		}
	} else {
		logger, found := loggerMap.Get(name)
		if found {
			logger.(*Logger).SetLogLevel(level)
		}
	}
}

func SetLogOut(out io.Writer) {
	currentLogOut = out
	for _, _logger := range loggerMap.Items() {
		logger := _logger.(*Logger)
		logger.Out = currentLogOut
	}
}

func ToggleLogToBuffer(enbale bool, size int64) {
	if enbale {
		if loggerBuffer != nil {
			return
		}
		var err error
		loggerBuffer, err = util.NewCircularBuffer(size)
		if err != nil {
			return
		}
		SetLogOut(loggerBuffer)
	} else {
		if loggerBuffer == nil {
			return
		}
		SetLogOut(defaultLogOut)
		loggerBuffer = nil
	}
}

func LogDump() []byte {
	if loggerBuffer == nil {
		return nil
	}

	return loggerBuffer.Bytes()
}

func GetLoggersInfo() []string {
	loggersInfo := make([]string, 0)
	for _, _logger := range loggerMap.Items() {
		logger := _logger.(*Logger)
		loggersInfo = append(loggersInfo, logger.String())
	}
	sort.Strings(loggersInfo)
	return loggersInfo
}

func NewLoggerEntry(moduleName string) *Entry {
	_logger, found := loggerMap.Get(moduleName)
	if found {
		logger := _logger.(*Logger)
		return logger.WithField("module", moduleName)
	}

	logger := &Logger{
		Name:           moduleName,
		isDefaultLevel: true,
		Level:          defaultLevel,
	}
	logger.Out = currentLogOut
	logger.Formatter = new(log.TextFormatter)
	logger.Hooks = make(log.LevelHooks)

	logger.SetLevel(getLevel(defaultLevel))
	for key, level := range globalLoggerMap {
		if key == "all" || key == "*" {
			logger.SetLogLevel(level)
			break
		} else if strings.HasSuffix(key, "*") {
			prefix := key[:len(key)-1]
			if strings.HasPrefix(moduleName, prefix) {
				logger.SetLogLevel(level)
				break
			}
		} else if key == moduleName {
			logger.SetLogLevel(level)
			break
		}
	}

	loggerMap.Set(moduleName, logger)
	return logger.WithField("module", moduleName)
}

func (logger *Logger) SetLogLevel(level Level) {
	if !isValidLevel(level) {
		return
	}

	logger.SetLevel(getLevel(level))
	logger.Level = level
	logger.isDefaultLevel = false
	log.New()
}

func (logger *Logger) String() string {
	return fmt.Sprintf("%s: %s", logger.Name, logger.Level)
}
