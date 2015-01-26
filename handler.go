/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2014/11/01 20:57:41
 */

package gologging

import (
    "io"
    "os"
    "time"
    "path/filepath"
    "strings"
    "strconv"
    "errors"
    "bytes"
    "fmt"
)

const (
    _OPEN_FILE_FLAG = os.O_WRONLY | os.O_CREATE | os.O_APPEND
    _OPEN_FILE_MODE = os.ModePerm & 0644
    _SUFFIX_SEP     = "_"
)

type _Msg struct {
    loggerName  string
    level       Level
    message     []byte
}

type Handler interface {
    Handle(loggerName string, level Level, message []byte) error
    SetFormatter(formatter *Formatter)
    SetLevel(level Level)
}

type HandlerLoop struct {
    q           chan *_Msg
    handler     Handler
}

func NewLoop(size int, handler Handler) *HandlerLoop {
    q := make(chan *_Msg, size)
    return &HandlerLoop{q, handler}
}

func (loop *HandlerLoop) HandleLoop() {
    for {
        msg := <-loop.q

        if err := loop.handler.Handle(msg.loggerName, msg.level, msg.message);
                err != nil {
            stdErrLog("failed to handle", err)
        }
    }
}

func (loop *HandlerLoop) Emit(msg *_Msg) {
    loop.q <- msg
}


// ------- StreamHandler ------- //
type StreamHandler struct {
    output      io.Writer
    formatter   *Formatter
    level       Level
}

func NewStreamHandle(out io.Writer) *StreamHandler {
    handler := StreamHandler{out, nil, INFO}

    return &handler
}

func (handler *StreamHandler) SetFormatter(formatter *Formatter) {
    handler.formatter = formatter
}

func (handler *StreamHandler) SetLevel(level Level) {
    handler.level = level
}

func (handler *StreamHandler) Handle(
        loggerName string,
        level Level,
        message []byte) error {
    if level < handler.level {
        return nil
    }

    if handler.formatter == nil {
        handler.formatter = NewFormatter(defautlFormatStr)
    }

    logMsg := handler.formatter.Format(loggerName, level, message)
    handler.output.Write(logMsg)

    return nil
}

func (handler *StreamHandler) SetOutput(out io.Writer) {
    handler.output = out
}


// ------- FileHandler ------- //
type FileHandler struct {
    StreamHandler
    fileName    string
    file        *os.File
    syncLog     bool
}

func NewFileHandler(fileName string) (*FileHandler, error) {
    file, err := os.OpenFile(fileName, _OPEN_FILE_FLAG, _OPEN_FILE_MODE)
    if err != nil {
        return nil, err
    }

    streamHandler := NewStreamHandle(file)
    handler := &FileHandler{StreamHandler: *streamHandler, fileName: fileName, file: file}

    return handler, nil
}

func (handler *FileHandler) Handle(
        loggerName string,
        level Level,
        message []byte) error {
    if err := handler.StreamHandler.Handle(loggerName, level, message); err != nil {
        return err
    }

    if handler.syncLog {
        if err := handler.file.Sync(); err != nil {
            return err
        }
    }

    return nil
}


type RotateInterval uint32

const (
    HOUR RotateInterval = 3600
    DAY RotateInterval = 86400
    MINUTE RotateInterval = 60
    HALF_HOUR RotateInterval = 1800
)


// ------- TimeRotateFileHandler ------- //
type TimeRotateFileHandler struct {
    FileHandler
    interval        RotateInterval
    lastLogTime     int64
    backupCount     uint16
}

func NewTimeRotateFileHandler(
        fileName string,
        interval RotateInterval,
        backupCount uint16) (*TimeRotateFileHandler, error) {
    fileHandler, err := NewFileHandler(fileName)
    if err != nil {
        return nil, err
    }

    rotateHandler := &TimeRotateFileHandler{
        FileHandler: *fileHandler,
        interval: interval,
        backupCount: backupCount,}

    return rotateHandler, nil
}

func (handler *TimeRotateFileHandler) Handle(
        loggerName string,
        level Level,
        message []byte) error {
    // Rotate file
    if handler.shouldRotate() {
        if err := handler.doRotate(); err != nil {
            return err
        }
    }

    return handler.FileHandler.Handle(loggerName, level, message)
}

func (handler *TimeRotateFileHandler) shouldRotate() bool {
    now := time.Now().Unix()
    if handler.lastLogTime == 0 {
        handler.lastLogTime = now
        return false
    }

    last := handler.lastLogTime / int64(handler.interval)
    cur := now / int64(handler.interval)

    handler.lastLogTime = now

    return cur > last
}

func (handler *TimeRotateFileHandler) doRotate() error {
    err := handler.file.Close()
    if err != nil {
        return err
    }

    // Make sure there are 'backupCount' logs at most
    files, err := filepath.Glob(handler.fileName + "_*")
    if err != nil {
        return err
    }

    if len(files) >= int(handler.backupCount) {
        rmCount := len(files) - int(handler.backupCount)
        // NOTICE: code below depends on the files order of filepath.Glob
        for i := 0; i < rmCount; i++ {
            if err := os.Remove(files[i]); err != nil {
                return err
            }
        }
    }

    // Time suffix
    suffix := time.Now().Format("200601021504")
    err = os.Rename(handler.fileName, handler.fileName + "_" + suffix)
    file, err := os.OpenFile(handler.fileName, _OPEN_FILE_FLAG, _OPEN_FILE_MODE)
    if err != nil {
        return err
    }
    handler.file = file
    handler.SetOutput(file)

    return nil
}

const (
    _ = iota
    KB int64 = 1 << (10 * iota)
    MB int64 = 1 << (10 * iota)
    GB int64 = 1 << (10 * iota)
)

// ------- SizeRotateFileHandler ------- //
type SizeRotateFileHandler struct {
    FileHandler
    maxBytes     int64
    curBytes     int64
    backupCount  uint16
    suffix       int32
}

func NewSizeRotateFileHandler(
        fileName string,
        maxBytes int64,
        backupCount uint16) (*SizeRotateFileHandler, error){
    fileHandler, err := NewFileHandler(fileName)
    if err != nil {
        return nil, err
    }

    // Fetch bytes for current log
    curBytes, err := fileHandler.file.Seek(0, 2)
    if err != nil {
        return nil, err
    }

    // Fetch max log suffix
    suffix, err := findMaxSuffix(fileName)
    if err != nil {
        return nil, err
    }

    rotateHandler := &SizeRotateFileHandler{
        FileHandler: *fileHandler,
        maxBytes: maxBytes,
        curBytes: curBytes,
        backupCount: backupCount,
        suffix: suffix,}

    return rotateHandler, nil
}

func (handler *SizeRotateFileHandler) Handle(
        loggerName string,
        level Level,
        message []byte) error {
    if level < handler.level {
        return nil
    }

    if handler.formatter == nil {
        handler.formatter = NewFormatter(defautlFormatStr)
    }

    logMsg := handler.formatter.Format(loggerName, level, message)
    handler.curBytes += int64(len(logMsg))
    // Need to rotate
    if handler.curBytes > handler.maxBytes {
        if err := handler.doRotate(); err != nil {
            return err
        }

        handler.curBytes = int64(len(logMsg))
    }

    wlen, err := handler.output.Write(logMsg)
    if err != nil || wlen != len(logMsg) {
        return errors.New("failed to Write logMsg")
    }

    if handler.syncLog {
        if err := handler.file.Sync(); err != nil {
            return err
        }
    }

    return nil
}

func (handler *SizeRotateFileHandler) doRotate() error {
    if err := handler.file.Close(); err != nil {
        return err
    }

    maxSuffix, err := findMaxSuffix(handler.fileName);
    if err != nil {
        return err
    }

    nameBuf := bytes.Buffer{}
    if maxSuffix >= int32(handler.backupCount) {
        // Make sure there are 'backupCount' logs at most
        sfn, dfn := bytes.Buffer{}, bytes.Buffer{}
        for i := 1; i < int(maxSuffix); i++ {
            sfn.Reset()
            dfn.Reset()
            fmt.Fprintf(&sfn, "%s_%04d", handler.fileName, i + 1)
            fmt.Fprintf(&dfn, "%s_%04d", handler.fileName, i)

            dfnStr, sfnStr := string(dfn.Bytes()), string(sfn.Bytes())
            if _, err := os.Stat(sfnStr); os.IsNotExist(err) {
                continue
            }

            if _, err := os.Stat(dfnStr); err == nil {
                if err := os.Remove(dfnStr); err != nil {
                    return err
                }
            }

            if err := os.Rename(sfnStr, dfnStr); err != nil {
                return err
            }
        }

        fmt.Fprintf(&nameBuf, "%s_%04d", handler.fileName, maxSuffix)
    } else {
        fmt.Fprintf(&nameBuf, "%s_%04d", handler.fileName, handler.suffix + 1)
        handler.suffix++
    }

    if err := os.Rename(handler.fileName, string(nameBuf.Bytes())); err != nil {
        return err
    }

    file, err := os.OpenFile(handler.fileName, _OPEN_FILE_FLAG, _OPEN_FILE_MODE)
    if err != nil {
        return err
    }

    handler.file = file
    handler.SetOutput(file)

    return nil
}

func findMaxSuffix(fileName string) (int32, error) {
    var maxSuffix int32 = 0
    files, err := filepath.Glob(fileName + "_*")
    if err != nil {
        return maxSuffix, err
    }

    for _, f := range files {
        sepIdx := strings.LastIndex(f, _SUFFIX_SEP)
        if sepIdx == -1 {
            continue
        }

        idx, err := strconv.Atoi(f[sepIdx+1:])
        if err != nil {
            return maxSuffix, err
        }

        if maxSuffix < int32(idx) {
            maxSuffix = int32(idx)
        }
    }

    return maxSuffix, nil
}

func defaultConsoleHandler() Handler {
    handler := NewStreamHandle(os.Stdout)
    handler.SetFormatter(NewFormatter(defautlFormatStr))
    return handler
}

func stdErrLog(msg string, err error) {
    buff := bytes.Buffer{}
    ts := time.Now().Format("2006-01-02 15:04:05")
    fmt.Fprintf(&buff, "%s %s err: %s\n", ts, msg, err.Error())
    os.Stderr.WriteString(string(buff.Bytes()))
}

