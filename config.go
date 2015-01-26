/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2014/12/02 17:24:10
 */

package gologging

import (
    "errors"
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
        return nil
    }

    if config.Handler != CONSOLE_HANDLER &&
            config.Name == "" {
        return errors.New("FileName required by TIME_ROTATE_HANDLER or SIZE_ROTATE_HANDLER")
    }

    // Create Handler
    var handler Handler
    var err error
    switch config.Handler {
    case CONSOLE_HANDLER:
        handler = defaultConsoleHandler()
    case TIME_ROTATE_HANDLER:
        handler, err = NewTimeRotateFileHandler(config.FileName, config.Interval, config.BackupCount)
    case SIZE_ROTATE_HANDLER:
        handler, err = NewSizeRotateFileHandler(config.FileName, config.MaxBytes, config.BackupCount)
    }

    if err != nil {
        return err
    }

    // Create Formatter
    if config.Format == "" {
        config.Format = defautlFormatStr
    }
    handler.SetFormatter(NewFormatter(config.Format))

    // Create Logger
    logger, ok := loggerMgr.logCache[config.Name]
    if !ok {
        logger = newLogger(config.Name, config.EnableConsoleLog)
        logger.SetLevel(config.LevelVal)
        loggerMgr.logCache[config.Name] = logger
    }
    logger.AddHandler(handler)

    return nil
}

