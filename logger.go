/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2014/10/31 15:06:24
 */

package gologging

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/chosen0ne/goutils"
	"os"
	"strings"
	"sync"
)

const (
	DEBUG = iota
	INFO
	WARN
	ERROR
	FATAL
	_MAX_LEVEL
	_DEFAULT_CHAN_SIZE = 100
)

var (
	loggerMgr *_LogMgr // use to manage all the loggers
)

var lvNames = []string{
	"DEBUG",
	"INFO",
	"WARN",
	"ERROR",
	"FATAL",
}

type Level int8

func NewLevelString(lvString string) Level {
	for idx, lvName := range lvNames {
		if lvName != strings.ToUpper(lvString) {
			continue
		}

		return Level(idx)
	}

	return _MAX_LEVEL
}

func (level Level) Name() string {
	if checkLevel(level) {
		return lvNames[level]
	}

	return "UNKOWN_LEVEL"
}

func (level Level) IsValid() bool {
	return level >= DEBUG && level < _MAX_LEVEL
}

type Logger struct {
	level Level
	name  string
	// Handlers will decide how to process the log message.
	// And a logger can be configure with multiple handlers.
	//
	// TODO: support sync mode to handle message
	handlers []*handlerLoop
}

func newLogger(name string, enableConsoleLog bool) *Logger {
	handlers := make([]*handlerLoop, 0)
	// Default logger level is INFO
	logger := &Logger{level: INFO, name: name, handlers: handlers}
	if enableConsoleLog {
		logger.AddHandler(defaultConsoleHandler())
	}

	return logger
}

func (logger *Logger) log(level Level, fmtStr string, vals ...interface{}) {
	if !checkLevel(level) {
		panic(errors.New("not support level"))
	}

	if level < logger.level {
		return
	}

	// Fill message
	msg := bytes.Buffer{}
	fmt.Fprintf(&msg, fmtStr, vals...)

	// Emit the message to all the handlers
	for _, handler := range logger.handlers {
		handler.Emit(&_Msg{loggerName: logger.name, level: level, message: msg.Bytes()})
	}
}

func (logger *Logger) debugInfo() string {
	b := &bytes.Buffer{}

	fmt.Fprintf(b, "name: %s\n", logger.name)
	fmt.Fprintf(b, "level: %s\n", logger.level.Name())
	fmt.Fprintln(b, "handlers:")
	for _, h := range logger.handlers {
		fmt.Fprintln(b, h.handler)
	}

	return string(b.Bytes())
}

func (logger *Logger) SetLevel(level Level) {
	if !checkLevel(level) {
		panic(errors.New("not support level"))
	}

	logger.level = level
}

func (logger *Logger) AddHandler(handler Handler) {
	loop := NewLoop(_DEFAULT_CHAN_SIZE, handler)
	logger.handlers = append(logger.handlers, loop)
	go loop.HandleLoop()
}

func (logger *Logger) Debug(fmt string, vals ...interface{}) {
	logger.log(DEBUG, fmt, vals...)
}

func (logger *Logger) Info(fmt string, vals ...interface{}) {
	logger.log(INFO, fmt, vals...)
}

func (logger *Logger) Warn(fmt string, vals ...interface{}) {
	logger.log(WARN, fmt, vals...)
}

func (logger *Logger) Error(fmt string, vals ...interface{}) {
	logger.log(ERROR, fmt, vals...)
}

func (logger *Logger) Exception(err error, fmt string, vals ...interface{}) {
	fmt = fmt + ", err: " + err.Error()
	logger.log(ERROR, fmt, vals...)
}

func (logger *Logger) Fatal(fmt string, vals ...interface{}) {
	logger.log(FATAL, fmt, vals...)
	os.Exit(1)
}

func checkLevel(level Level) bool {
	return level >= DEBUG && level < _MAX_LEVEL
}

type _LogMgr struct {
	logCache   map[string]*Logger // name => Logger
	mu         sync.Mutex         // Synchronize logCache map.
	rootLogger *Logger
}

func (mgr *_LogMgr) GetLogger(name string) *Logger {
	return mgr.getLogger(name, true)
}

func (mgr *_LogMgr) getLogger(name string, enableConsoleLog bool) *Logger {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	logger, ok := mgr.logCache[name]
	if !ok {
		logger = newLogger(name, enableConsoleLog)
		mgr.logCache[name] = logger
	}

	return logger
}

func (mgr *_LogMgr) AddLogger(name string, logger *Logger) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if _, ok := mgr.logCache[name]; !ok {
		mgr.logCache[name] = logger
	} else {
		return goutils.NewErr("logger named '%s' already exists", name)
	}

	return nil
}

// Methods for root logger, all the message emit by root logger
// wil be output to stdout
func Debug(fmt string, vals ...interface{}) {
	loggerMgr.rootLogger.Debug(fmt, vals...)
}

func Info(fmt string, vals ...interface{}) {
	loggerMgr.rootLogger.Info(fmt, vals...)
}

func Warn(fmt string, vals ...interface{}) {
	loggerMgr.rootLogger.Warn(fmt, vals...)
}

func Error(fmt string, vals ...interface{}) {
	loggerMgr.rootLogger.Error(fmt, vals...)
}

func Exception(err error, fmt string, vals ...interface{}) {
	loggerMgr.rootLogger.Exception(err, fmt, vals...)
}

func Fatal(fmt string, vals ...interface{}) {
	loggerMgr.rootLogger.Fatal(fmt, vals...)
}

func GetLogger(name string) *Logger {
	return loggerMgr.GetLogger(name)
}

// ConfigSizeRotateLogger configure a size rotated logger
func ConfigSizeRotateLogger(
	name string,
	level Level,
	maxBytes int64,
	backupCount uint16,
	enableConsoleLog bool) error {

	logger := loggerMgr.getLogger(name, false)
	handler, err := NewSizeRotateFileHandler(name+".log", maxBytes, backupCount)
	if err != nil {
		return err
	}
	logger.AddHandler(handler)
	logger.SetLevel(level)

	if enableConsoleLog {
		logger.AddHandler(defaultConsoleHandler())
	}

	return nil
}

// ConfigTimeRotateLogger configures a size rotated logger
func ConfigTimeRotateLogger(
	name string,
	level Level,
	interval RotateInterval,
	backupCount uint16,
	enableConsoleLog bool) error {

	logger := loggerMgr.getLogger(name, false)
	handler, err := NewTimeRotateFileHandler(name+".log", interval, backupCount)
	if err != nil {
		return err
	}
	logger.AddHandler(handler)
	logger.SetLevel(level)

	if enableConsoleLog {
		logger.AddHandler(defaultConsoleHandler())
	}

	return nil
}

func init() {
	loggerMgr = &_LogMgr{}
	loggerMgr.logCache = make(map[string]*Logger)
	loggerMgr.rootLogger = newLogger("root", true)
}
