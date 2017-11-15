/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2017-11-02 16:15:27
 */

package gologging

import (
	"github.com/chosen0ne/goconf"
	"github.com/chosen0ne/goutils"
	"strconv"
	"strings"
)

const (
	_HANDLER_EXTEND     = "extends"
	_LOGGERS_LABEL      = "loggers"
	_LEVEL_LABEL        = "level"
	_HANDLERS_LABEL     = "handlers"
	_TYPE_LABEL         = "type"
	_MAX_SIZE_LABEL     = "max-size"
	_INTERVAL_LABEL     = "interval"
	_SYNC_MODE_LABEL    = "sync"
	_LOG_PATH_LABEL     = "log-path"
	_FORMMATER_LABEL    = "formatter"
	_BACKUP_COUNT_LABEL = "backup-count"
	_FILENAME_LABEL     = "file-name"
	_FORMAT_LABEL       = "format"
)

var (
	handlerTypes  map[string]HandlerType
	intervalTypes map[string]RotateInterval
	sizeTypes     map[string]int64
)

// To avoid to create object repeatedly. Use context to reuse objects.
// Type mappings in the context are:
//		handler   -> LoggerConfig
//		extend    -> LoggerConfig
//		formmater -> string
type context map[string]interface{}

func newContext() context {
	return make(map[string]interface{})
}

// Configure loggers base on file.
func Load(configPath string) error {
	conf := goconf.New(configPath)
	if err := conf.Parse(); err != nil {
		return goutils.WrapErrorf(err, "failed to parse conf, conf: %s", configPath)
	}

	loggerNames, err := conf.GetStringArray(_LOGGERS_LABEL)
	if err != nil {
		return goutils.WrapErrorf(err, "failed to get loggers from config")
	}

	ctx := newContext()
	for _, loggerName := range loggerNames {
		if !isValidLogger(loggerName) {
			return goutils.NewErr("invalid section name for logger, name: %s", loggerName)
		}

		if err := loadLogger(loggerName, conf, ctx); err != nil {
			return goutils.WrapErrorf(err, "failed to load logger, name: %s", loggerName)
		}
	}

	return nil
}

func isValidLogger(loggerName string) bool {
	// Config option name for logger must start with 'logger-'
	return strings.HasPrefix(loggerName, "logger-")
}

func loadLogger(loggerName string, conf *goconf.Conf, ctx context) error {
	if err := conf.Section(loggerName); err != nil {
		return goutils.WrapErrorf(err, "failed to go to section, section: %s", loggerName)
	}

	if !conf.HasItem(_HANDLERS_LABEL) {
		return goutils.NewErr("logger has no handlers, name: %s", loggerName)
	}

	// parse level
	var level Level
	if !conf.HasItem(_LEVEL_LABEL) {
		level = INFO
	} else {
		if lvStr, err := conf.GetString(_LEVEL_LABEL); err != nil {
			return goutils.WrapErrorf(err, "failed to get level config")
		} else {
			if level = NewLevelString(lvStr); level == _MAX_LEVEL {
				return goutils.NewErr("Unkown logger level, level: %s", lvStr)
			}
		}
	}

	// config name for logger: logger-${LOGGER-NAME}
	fields := strings.SplitN(loggerName, "-", 2)
	logger := newLogger(fields[1], false)

	// load handlers
	handlerNames, err := conf.GetStringArray(_HANDLERS_LABEL)
	if err != nil {
		return goutils.WrapErrorf(err, "failed to get handlers config")
	}
	for _, handlerName := range handlerNames {
		if handlerConfig, err := loadHandler(handlerName, conf, ctx); err != nil {
			return goutils.WrapErrorf(err, "failed to load handler, name: %s", handlerName)
		} else {
			if handler, err := createHandler(handlerConfig); err != nil {
				return goutils.WrapErrorf(err, "failed to create handler, name: %s", handlerName)
			} else {
				logger.AddHandler(handler)
			}
		}
	}

	if err := loggerMgr.AddLogger(fields[1], logger); err != nil {
		return goutils.WrapErrorf(err, "failed to add logger to LogMgr, name: %s", fields[1])
	}

	return nil
}

func loadHandler(handlerName string, conf *goconf.Conf, ctx context) (*LoggerConfig, error) {
	// fetch object from context at frist
	if handlerConfObj, ok := ctx[handlerName]; ok {
		if handlerConf, assertOk := handlerConfObj.(*LoggerConfig); assertOk {
			return handlerConf, nil
		} else {
			return nil, goutils.NewErr("config for handler named '%s' in context isn't a *LoggerConfig",
				handlerName)
		}
	}

	if err := conf.Section(handlerName); err != nil {
		return nil, goutils.WrapErrorf(err, "failed to go section, section: %s", handlerName)
	}

	var handlerConf *LoggerConfig

	if conf.HasItem(_HANDLER_EXTEND) {
		if extName, err := conf.GetString(_HANDLER_EXTEND); err != nil {
			return nil, goutils.WrapErrorf(err, "failed to get extend from config")
		} else if loggerConfig, err := loadExtendConfig(extName, conf, ctx); err != nil {
			return nil, goutils.WrapErrorf(err, "failed to load extend config, extend name: %s",
				extName)
		} else {
			handlerConf = newLoggerConfig(loggerConfig)
			ctx[handlerName] = handlerConf
		}

		// Back to section of handler config
		// No need to check errors, because error check has been done at the start of the method.
		conf.Section(handlerName)
	} else {
		handlerConf = &LoggerConfig{}
	}

	if err := loadLoggerConfig(handlerConf, conf, ctx); err != nil {
		return nil, goutils.WrapErrorf(err, "failed to load config in handler, handler: %s", handlerName)
	}

	// add to context
	ctx[handlerName] = handlerConf

	return handlerConf, nil
}

func loadExtendConfig(extName string, conf *goconf.Conf, ctx context) (*LoggerConfig, error) {
	if configObj, ok := ctx[extName]; ok {
		if config, assertOk := configObj.(*LoggerConfig); assertOk {
			return config, nil
		} else {
			return nil, goutils.NewErr("config named '%s' in context isn't a *LoggerConfig", extName)
		}
	}

	if err := conf.Section(extName); err != nil {
		return nil, goutils.WrapErrorf(err, "failed to turn to section, section: %s", extName)
	}

	configObj := &LoggerConfig{}
	if err := loadLoggerConfig(configObj, conf, ctx); err != nil {
		return nil, goutils.WrapErrorf(err, "failed to load logger config, extend name: %s", extName)
	}

	// add to context
	ctx[extName] = configObj

	return configObj, nil
}

func loadLoggerConfig(configObj *LoggerConfig, conf *goconf.Conf, ctx context) error {
	if configObj == nil {
		return goutils.NewErr("invalid param, configObj is nil")
	}

	if configObj.Handler == "" {
		// Type of handler has not been set
		if htStr, err := conf.GetString(_TYPE_LABEL); err != nil {
			return goutils.WrapErrorf(err, "failed to get config, name: %s", _TYPE_LABEL)
		} else if _, ok := handlerTypes[htStr]; !ok {
			return goutils.NewErr("unknown handler type: %s", htStr)
		} else {
			configObj.Handler, _ = handlerTypes[htStr]
		}
	}

	var err error
	if conf.HasItem(_INTERVAL_LABEL) {
		if configObj.Interval, err = parseInterval(conf); err != nil {
			return goutils.WrapErrorf(err, "failed to parse interval")
		}
	}

	if conf.HasItem(_MAX_SIZE_LABEL) {
		if configObj.MaxBytes, err = parseSize(conf); err != nil {
			return goutils.WrapErrorf(err, "failed to parse max size")
		}
	}

	if conf.HasItem(_BACKUP_COUNT_LABEL) {
		bakupCount, err := conf.GetInt(_BACKUP_COUNT_LABEL)
		if err != nil {
			return goutils.WrapErrorf(err, "failed to parse back up count")
		}
		configObj.BackupCount = uint16(bakupCount)
	}

	if conf.HasItem(_SYNC_MODE_LABEL) {
		configObj.SyncMode, err = parseSyncMode(conf)
		if err != nil {
			return goutils.WrapErrorf(err, "failed to parse sync mode")
		}

		configObj.SyncWrite = configObj.SyncMode
	}

	if conf.HasItem(_LOG_PATH_LABEL) {
		configObj.LogPath, err = conf.GetString(_LOG_PATH_LABEL)
		if err != nil {
			return goutils.WrapErrorf(err, "failed to parse log path")
		}
	}

	// format
	if conf.HasItem(_FORMMATER_LABEL) {
		if fmtName, err := conf.GetString(_FORMMATER_LABEL); err != nil {
			return goutils.WrapErrorf(err, "failed to get formatter from config")
		} else if fmtStr, err := loadFormatter(fmtName, conf, ctx); err != nil {
			return goutils.WrapErrorf(err, "failed to load formatter, name: %s", fmtName)
		} else {
			configObj.Format = fmtStr
		}
	}

	if conf.HasItem(_FILENAME_LABEL) {
		if fname, err := conf.GetString(_FILENAME_LABEL); err != nil {
			return goutils.WrapErrorf(err, "failed to get file name from config")
		} else {
			configObj.FileName = fname
		}
	}

	return nil
}

func loadFormatter(fmtName string, conf *goconf.Conf, ctx context) (string, error) {
	if fmtObj, ok := ctx[fmtName]; ok {
		if fmtStr, assertOk := fmtObj.(string); assertOk {
			return fmtStr, nil
		} else {
			return "", goutils.NewErr("object for formatter in context is't a string, formatter: %s",
				fmtName)
		}
	}

	if !conf.HasSection(fmtName) {
		return "", goutils.NewErr("no formatter named '%s'", fmtName)
	}

	if err := conf.Section(fmtName); err != nil {
		return "", goutils.WrapErrorf(err, "failed to go to section, name: %s", fmtName)
	}

	if fmtStr, err := conf.GetString(_FORMAT_LABEL); err != nil {
		return "", goutils.WrapErrorf(err, "failed to get format from config")
	} else {
		ctx[fmtName] = fmtStr

		return fmtStr, nil
	}
}

func parseInterval(conf *goconf.Conf) (RotateInterval, error) {
	intervalStr, err := conf.GetString(_INTERVAL_LABEL)
	if err != nil {
		return 0, goutils.WrapErrorf(err, "failed to get conifg, name: %s", _INTERVAL_LABEL)
	}

	if num, unitStr, err := splitNumAndStr(intervalStr); err != nil {
		return 0, goutils.WrapErrorf(err, "failed to split, value: %s", intervalStr)
	} else {
		unit, ok := intervalTypes[strings.ToLower(unitStr)]
		if !ok {
			return 0, goutils.NewErr("unknown interval type: %s", unitStr)
		}
		return RotateInterval(uint32(num) * uint32(unit)), nil
	}
}

func parseSize(conf *goconf.Conf) (int64, error) {
	sizeStr, err := conf.GetString(_MAX_SIZE_LABEL)
	if err != nil {
		return 0, goutils.WrapErrorf(err, "failed to get max size from config")
	}

	if num, unitStr, err := splitNumAndStr(sizeStr); err != nil {
		return 0, goutils.WrapErrorf(err, "failed to split max size")
	} else {
		unit, ok := sizeTypes[strings.ToUpper(unitStr)]
		if !ok {
			return 0, goutils.NewErr("unknown size type: %s", unitStr)
		}
		return int64(num) * unit, nil
	}
}

func parseSyncMode(conf *goconf.Conf) (bool, error) {
	syncMode, err := conf.GetString(_SYNC_MODE_LABEL)
	if err != nil {
		return false, goutils.WrapErrorf(err, "failed to get sync mode from config")
	}

	return strings.ToLower(syncMode) == "true", nil
}

// input: "100day"
// return: 100, "day", nil
func splitNumAndStr(s string) (int, string, error) {
	var idx int
	for i, c := range s {
		if c < '0' || c > '9' {
			idx = i
			break
		}
	}

	num, err := strconv.Atoi(s[:idx])
	if err != nil {
		return 0, "", goutils.WrapErrorf(err, "failed converto int, idx: %d", idx)
	}

	return num, s[idx:], nil
}

func init() {
	handlerTypes = map[string]HandlerType{
		"console":     CONSOLE_HANDLER,
		"time-rotate": TIME_ROTATE_HANDLER,
		"size-rotate": SIZE_ROTATE_HANDLER,
	}

	intervalTypes = map[string]RotateInterval{
		"day":  DAY,
		"min":  MINUTE,
		"hour": HOUR,
	}

	sizeTypes = map[string]int64{
		"KB": KB,
		"MB": MB,
		"GB": GB,
	}
}
