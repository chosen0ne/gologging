/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2014/10/31 15:06:24
 */

package gologging

import (
    "sync"
    "bytes"
    "fmt"
    "errors"
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
    loggerMgr   *_LogMgr        // use to manage all the loggers
)

var lvNames = []string {
    "DEBUG",
    "INFO",
    "WARN",
    "ERROR",
    "FATAL",
}

type Level int8

func (level Level) Name() string {
    if checkLevel(level) {
        return lvNames[level]
    }

    return "UNKOWN_LEVEL"
}

type Logger struct {
    level       Level
    name        string
    handlers    []*HandlerLoop
    mu          sync.Mutex
}

func newLogger(name string, enableConsoleLog bool) *Logger {
    // Default console log handler
    handlers := make([]*HandlerLoop, 0)
    logger := &Logger{level: INFO, name: name, handlers: handlers}
    if enableConsoleLog {
        logger.AddHandler(defaultConsoleHandler())
    }

    // Default log level is INFO
    return logger
}

func (logger *Logger) Log(level Level, fmtStr string, vals ...interface{}) {
    if !checkLevel(level) {
        panic(errors.New("not support level"))
    }

    if level < logger.level {
        return
    }

    // Fill message
    msg := bytes.Buffer{}
    fmt.Fprintf(&msg, fmtStr, vals...)

    for _, handler := range logger.handlers {
        handler.Emit(&_Msg{logger.name, level, msg.Bytes()})
    }
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
    logger.Log(DEBUG, fmt, vals...)
}

func (logger *Logger) Info(fmt string, vals ...interface{}) {
    logger.Log(INFO, fmt, vals...)
}

func (logger *Logger) Warn(fmt string, vals ...interface{}) {
    logger.Log(WARN, fmt, vals...)
}

func (logger *Logger) Error(fmt string, vals ...interface{}) {
    logger.Log(ERROR, fmt, vals...)
}

func (logger *Logger) Fatal(fmt string, vals ...interface{}) {
    logger.Log(FATAL, fmt, vals...)
}

func checkLevel(level Level) bool {
    return level >= DEBUG && level < _MAX_LEVEL
}

// ------- Private class ------- //
type _LogMgr struct {
    logCache    map[string]*Logger
    mu          sync.Mutex
    rootLogger  *Logger
}

func (mgr *_LogMgr) GetLogger(name string) *Logger {
    mgr.mu.Lock()
    defer mgr.mu.Unlock()

    logger, ok := mgr.logCache[name]
    if !ok {
        logger = newLogger(name, true)
        mgr.logCache[name] = logger
    }

    return logger
}

// ------- Log method for root logger ------- //
// root logger emit log to std out
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

func Fatal(fmt string, vals ...interface{}) {
    loggerMgr.rootLogger.Fatal(fmt, vals...)
}

func GetLogger(name string) *Logger {
    return loggerMgr.GetLogger(name)
}

func ConfigSizeRotateLogger(
        name string,
        maxBytes int64,
        backupCount uint16) error {
    if _, ok := loggerMgr.logCache[name]; ok {
        return errors.New("logger named '" + name + "' already exists!")
    }

    logger := loggerMgr.GetLogger(name)
    handler, err := NewSizeRotateFileHandler(name + ".log", maxBytes, backupCount)
    if err != nil {
        return err
    }
    logger.AddHandler(handler)

    return nil
}

func ConfigTimeRotateLogger(
        name string,
        interval RotateInterval,
        backupCount uint16) error {
    if _, ok := loggerMgr.logCache[name]; ok {
        return errors.New("logger named '" + name + "' already exists!")
    }
    logger := loggerMgr.GetLogger(name)
    hander, err := NewTimeRotateFileHandler(name + ".log", interval, backupCount)
    if err != nil {
        return err
    }
    logger.AddHandler(hander)

    return nil
}

func init() {
    loggerMgr = &_LogMgr{}
    loggerMgr.logCache = make(map[string]*Logger)
    loggerMgr.rootLogger = newLogger("root", true)
}
