/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2014/12/02 17:24:10
 */

package gologging

import (
    "errors"
    "strings"
    "path"
    "os"
)

type HandlerType string

const (
    CONSOLE_HANDLER HandlerType = "ConsoleHandler"
    TIME_ROTATE_HANDLER HandlerType = "TimeRotateFileHandler"
    SIZE_ROTATE_HANDLER HandlerType = "SizeRotateFileHandler"
)

func (ht HandlerType) Name() string {
    return string(ht)
}

func validHandlerType(ht HandlerType) bool {
    if ht == CONSOLE_HANDLER || ht == TIME_ROTATE_HANDLER ||
            ht == SIZE_ROTATE_HANDLER {
        return true
    }

    return false
}

type LoggerConfig struct {
    Name                string
    LevelVal            Level
    Format              string
    Handler             HandlerType
    Interval            RotateInterval
    BackupCount         uint16
    MaxBytes            int64
    FileName            string
    EnableConsoleLog    bool
    LogPath             string
}

func defaultValIfNonExist(config *LoggerConfig) {
    if config.Format == "" {
        config.Format = defautlFormatStr
    }
    if config.Interval == 0 {
        config.Interval = DAY
    }
    if config.BackupCount == 0 {
        config.BackupCount = 10
    }
    if config.MaxBytes == 0 {
        config.MaxBytes = 100 * MB
    }
}

func ConfigLogger(config *LoggerConfig) error {
    if !validHandlerType(config.Handler) {
        return errors.New("not support handler: " + string(config.Handler))
    }

    if config.Name == "" {
        return errors.New("logger Name required")
    }

    defaultValIfNonExist(config)

    loggerMgr.mu.Lock()
    defer loggerMgr.mu.Unlock()

    // Each Logger can only be initialize once
    if _, ok := loggerMgr.logCache[config.Name]; ok {
        return errors.New("logger named '" + config.Name + "' already exists!")
    }

    if config.Handler != CONSOLE_HANDLER &&
            config.FileName == "" {
        config.FileName = config.Name
    }

    if !strings.HasSuffix(config.FileName, ".log") {
        config.FileName += ".log"
    }

    if config.LogPath == "" {
        config.LogPath = "."
    }

    var err error
    fpath, err := getAbsPath(config.LogPath, config.FileName)
    if err != nil {
        return err
    }

    // Create Handler
    var handler Handler
    switch config.Handler {
    case CONSOLE_HANDLER:
        handler = defaultConsoleHandler()
        config.EnableConsoleLog = false
    case TIME_ROTATE_HANDLER:
        handler, err = NewTimeRotateFileHandler(fpath, config.Interval, config.BackupCount)
    case SIZE_ROTATE_HANDLER:
        handler, err = NewSizeRotateFileHandler(fpath, config.MaxBytes, config.BackupCount)
    }

    if err != nil {
        return err
    }

    // Create Formatter
    if config.Format == "" {
        config.Format = defautlFormatStr
    }

    formatter, err := NewFormatter(config.Format)
    if err != nil {
        return err
    }
    handler.SetFormatter(formatter)

    // Create Logger
    logger, ok := loggerMgr.logCache[config.Name]
    if !ok {
        logger = newLogger(config.Name, config.EnableConsoleLog)
        loggerMgr.logCache[config.Name] = logger
    }
    logger.SetLevel(config.LevelVal)
    logger.AddHandler(handler)

    return nil
}

func getAbsPath(fpath, fname string) (string, error) {
    if path.IsAbs(fpath) {
        return path.Join(fpath, fname), nil
    }

    dir, err := os.Getwd();
    if err != nil {
        return "", err
    }

    return path.Join(dir, fname), nil
}
