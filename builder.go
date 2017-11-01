/**
 * Use Builder design pattern to config a logger.
 * TimeRotate or SizeRotate must be called firstly,
 * and Config must be called finally.
 * e.g.
 *     gologging.TimeRotate().Interval(gologging.HOUR).BackupCount(5)
 *	           .Config("time-rotate-a", "time-rotate-b", "some-other")
 *	   gologging.SizeRotate().MaxBytes(gologging.MB * 10).BackupCount(5)
 *			   .Config("size-rotate")
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2017-03-16 18:59:01
 */

package gologging

type loggerBuilder struct {
	config LoggerConfig
	// When multi-logger is configured together by Config(), 'useSameFile'
	// will specify whether all the loggers use the same file to output.
	// It will be 'true', when File() is called.
	useSameFile bool
}

func TimeRotate() *loggerBuilder {
	return builder(TIME_ROTATE_HANDLER)
}

func SizeRotate() *loggerBuilder {
	return builder(SIZE_ROTATE_HANDLER)
}

func builder(handlerType HandlerType) *loggerBuilder {
	if !validHandlerType(handlerType) {
		panic("not support handler: " + handlerType)
	}

	conf := LoggerConfig{
		INFO,
		defautlFormatStr,
		handlerType,
		DAY,
		10,
		100 * MB,
		"",
		true,
		"./",
		false,
	}

	return &loggerBuilder{conf, false}
}

func (b *loggerBuilder) Level(lv Level) *loggerBuilder {
	b.config.LevelVal = lv
	return b
}

func (b *loggerBuilder) Format(f string) *loggerBuilder {
	b.config.Format = f
	return b
}

func (b *loggerBuilder) Interval(i RotateInterval) *loggerBuilder {
	if b.config.Handler != TIME_ROTATE_HANDLER {
		panic("'Interval' is only used by time rotated handler")
	}
	b.config.Interval = i
	return b
}

func (b *loggerBuilder) BackupCount(count uint16) *loggerBuilder {
	b.config.BackupCount = count
	return b
}

func (b *loggerBuilder) MaxBytes(maxBytes int64) *loggerBuilder {
	if b.config.Handler != SIZE_ROTATE_HANDLER {
		panic("'MaxBytes' is only used by size rotated handler")
	}
	return b
}

func (b *loggerBuilder) FileName(fname string) *loggerBuilder {
	b.config.FileName = fname
	b.useSameFile = true
	return b
}

func (b *loggerBuilder) ConsoleLog(enable bool) *loggerBuilder {
	b.config.EnableConsoleLog = enable
	return b
}

func (b *loggerBuilder) LogPath(path string) *loggerBuilder {
	b.config.LogPath = path
	return b
}

func (b *loggerBuilder) SyncWrite(sync bool) *loggerBuilder {
	b.config.SyncWrite = sync
	return b
}

// Config will conifgure the logger by previous config.
// And it can be used to configure mulitiple loggers.
func (b *loggerBuilder) Config(names ...string) {
	for _, name := range names {
		if !b.useSameFile {
			b.config.FileName = ""
		}
		if err := ConfigLogger(name, &b.config); err != nil {
			panic("failed to config log, name: " + name + ", err: " + err.Error())
		}
	}
}
