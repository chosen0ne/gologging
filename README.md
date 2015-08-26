# gologging
=============================
logging for golang

###1. Features
- 支持格式化字符串
- 延迟字符串格式化，只在真正需要打印日志时才进行
- 支持按时间、大小切分日志
- 支持控制台日志输出

###2. 内置内置格式化tag
- ${date}: 日期
- ${time}: 时间
- ${datetime}: 日期和时间
- ${funcname}: 所在函数名
- ${filename}: 文件名
- ${filepath}: 文件所在路径
- ${lineno}: 日志打印所在行号
- ${levelname}: 日志级别
- ${name}: logger name
- ${message}: 文本日志

###3. Sample Code
####1) 按时间切分
    ConfigTimeRotateLogger(
        'time-rotate',  // name: logger name
        INFO,           // level: log level
        HOUR * 6,       // interval: 切分时间间隔
        10,             // backupCount: 日志备份个数
        false)          // enableConsoleLog: 是否开启控制台日志输出
    logger := GetLogger("time-rotate")
    logger.Info("a time-rotate log, num: %d, str: %s", 10, "some string")

####2) 按大小切分
    ConfigSizeRotateLogger(
        'size-rotate',  // name: logger name
        INFO,           // level: log level
        100 * MB,       // maxBytes: 单个日志的最大大小
        10,             // backupCount: 日志备份个数
        true)           // enableConsoleLog: 是否开启控制台日志输出
    logger := GetLogger("size-rotate")
    logger.Info("a size-rotate log, size: %d, tag: %s", 1024, "some tag")

####3) 通过配置对象初始化
    config := LoggerConfig{
        Name: "log-name",
        LevelVal: INFO,
        Format: "${datetime}-${levelname}-${message}",
        Handler: TIME_ROTATE_HANDLER,
        Interval: 2 * HOUR,
        FileName: "log-name",
        EnableConsoleLog: true,
        LogPath: "/home/logs/"
    }
    ConfigLogger(&config)
    logger := GetLogger("log-name")

