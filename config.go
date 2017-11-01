/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2014/12/02 17:24:10
 */

package gologging

import (
	"errors"
	"os"
	"path"
	"strings"
)

type HandlerType string

const (
	CONSOLE_HANDLER     HandlerType = "ConsoleHandler"
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
	LevelVal         Level
	Format           string
	Handler          HandlerType
	Interval         RotateInterval
	BackupCount      uint16
	MaxBytes         int64
	FileName         string
	EnableConsoleLog bool
	LogPath          string
	// Wheather 'sync' data to file after each log write,
	// that make sure logs can not be lost.
	SyncWrite bool
}

func ConfigLogger(name string, config *LoggerConfig) error {
	if !validHandlerType(config.Handler) {
		return errors.New("not support handler: " + string(config.Handler))
	}

	setDefaultConfig(name, config)

	loggerMgr.mu.Lock()
	defer loggerMgr.mu.Unlock()

	// Each Logger can only be initialize once
	if _, ok := loggerMgr.logCache[name]; ok {
		return errors.New("logger named '" + name + "' already exists!")
	}

	handler, err := createHandler(config)
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
	logger, ok := loggerMgr.logCache[name]
	if !ok {
		logger = newLogger(name, config.EnableConsoleLog)
		loggerMgr.logCache[name] = logger
	}
	logger.SetLevel(config.LevelVal)
	logger.AddHandler(handler)

	return nil
}

func setDefaultConfig(name string, config *LoggerConfig) {
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
	if config.Handler != CONSOLE_HANDLER && config.FileName == "" {
		config.FileName = name
	}
	if !strings.HasSuffix(config.FileName, ".log") {
		config.FileName += ".log"
	}
	if config.LogPath == "" {
		config.LogPath = "."
	}
}

func createHandler(config *LoggerConfig) (Handler, error) {
	fpath, err := getAbsPath(config.LogPath, config.FileName)
	if err != nil {
		return nil, err
	}

	var handler Handler
	switch config.Handler {
	case CONSOLE_HANDLER:
		handler = defaultConsoleHandler()
		config.EnableConsoleLog = false

	case TIME_ROTATE_HANDLER:
		h, err := NewTimeRotateFileHandler(fpath, config.Interval, config.BackupCount)
		if err != nil {
			return nil, err
		}
		handler = h
		h.FileHandler.SyncWrite(config.SyncWrite)

	case SIZE_ROTATE_HANDLER:
		h, err := NewSizeRotateFileHandler(fpath, config.MaxBytes, config.BackupCount)
		if err != nil {
			return nil, err
		}
		handler = h
		h.FileHandler.SyncWrite(config.SyncWrite)
	}

	return handler, nil
}

func getAbsPath(fpath, fname string) (string, error) {
	if path.IsAbs(fpath) {
		return path.Join(fpath, fname), nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return path.Join(path.Join(dir, fpath), fname), nil
}
