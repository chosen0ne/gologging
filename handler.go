/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2014/11/01 20:57:41
 */

package gologging

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	innerFuncNames map[string]int
)

const (
	_OPEN_FILE_FLAG = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	_OPEN_FILE_MODE = os.ModePerm & 0644
	_SUFFIX_SEP     = "_"
	_CALLER_SKIP    = 3
	_CALLER_SIZE    = 10
)

type _Msg struct {
	loggerName string
	level      Level
	message    []byte
	funcName   string
	fileName   string
	lineNo     int
}

// Interface to handle each log message.
type Handler interface {
	Handle(msg *_Msg) error
	SetFormatter(formatter *Formatter)
	SetLevel(level Level)
	SetSyncMode(sync bool)
	IsSync() bool // wheather or not to synchronize log 'Emit' and 'Handle'
}

// As some handlers need to do some high-cost things when
// log is emitted, such as file rotation. So we made handlers
// run in a goroutine that process things asynchronously,
// which can not affect the main goroutine.
type handlerLoop struct {
	q       chan *_Msg
	handler Handler
	w       chan byte // used to make sure 'Emit' and 'Handle' are synchronous.
}

func NewLoop(size int, handler Handler) *handlerLoop {
	q := make(chan *_Msg, size)
	return &handlerLoop{q, handler, make(chan byte)}
}

func (loop *handlerLoop) HandleLoop() {
	for {
		msg := <-loop.q

		if err := loop.handler.Handle(msg); err != nil {
			stdErrLog("failed to handle", err)
		}

		if loop.handler.IsSync() {
			// notify the goroutine which invoked 'Emit'
			loop.w <- '0'
		}
	}
}

func (loop *handlerLoop) Emit(msg *_Msg) {
	stacks := make([]uintptr, _CALLER_SIZE)
	n := runtime.Callers(_CALLER_SKIP, stacks)

	// Find the first function which is not owned by gologging
	for i := 0; i < n; i++ {
		fn := runtime.FuncForPC(stacks[i])
		funcName := fn.Name()
		if loop.isExternalFunc(funcName) {
			fileName, lineNo := fn.FileLine(stacks[i])
			msg.fileName, msg.lineNo, msg.funcName = fileName, lineNo, funcName
			break
		}
	}

	loop.q <- msg

	if loop.handler.IsSync() {
		// wait for finish of handle
		<-loop.w
	}
}

func (loop *handlerLoop) isExternalFunc(funcName string) bool {
	parts := strings.Split(funcName, "/")
	if len(parts) <= 0 {
		return false
	}

	_, ok := innerFuncNames[parts[len(parts)-1]]

	return !ok
}

// A log handler used to emit message to stream.
type StreamHandler struct {
	output    io.Writer
	formatter *Formatter
	level     Level
	isSync    bool
}

func NewStreamHandle(out io.Writer) *StreamHandler {
	handler := StreamHandler{out, nil, INFO, false}

	return &handler
}

func (handler *StreamHandler) SetFormatter(formatter *Formatter) {
	handler.formatter = formatter
}

func (handler *StreamHandler) SetLevel(level Level) {
	handler.level = level
}

func (handler *StreamHandler) Handle(msg *_Msg) error {
	if msg.level < handler.level {
		return nil
	}

	if handler.formatter == nil {
		handler.formatter, _ = NewFormatter(defautlFormatStr)
	}

	logMsg := handler.formatter.Format(msg)
	handler.output.Write(logMsg)

	return nil
}

func (handler *StreamHandler) SetSyncMode(sync bool) {
	handler.isSync = sync
}

func (handler *StreamHandler) IsSync() bool {
	return handler.isSync
}

func (handler *StreamHandler) SetOutput(out io.Writer) {
	handler.output = out
}

func (handler *StreamHandler) String() string {
	var b bytes.Buffer

	fmt.Fprintf(&b, "StreamHandler{level: %s, isSync: %t}", handler.level.Name(), handler.isSync)

	return string(b.Bytes())
}

// A log handler which emits message to a file, and it's
// derived from StreamHandler.
type FileHandler struct {
	StreamHandler
	fileName    string
	file        *os.File
	isSyncWrite bool
}

func NewFileHandler(fileName string) (*FileHandler, error) {
	file, err := os.OpenFile(fileName, _OPEN_FILE_FLAG, _OPEN_FILE_MODE)
	if err != nil {
		return nil, err
	}

	streamHandler := NewStreamHandle(file)
	handler := &FileHandler{
		StreamHandler: *streamHandler,
		fileName:      fileName,
		file:          file,
	}

	return handler, nil
}

func (handler *FileHandler) SyncWrite(isSync bool) {
	handler.isSyncWrite = isSync
}

func (handler *FileHandler) Handle(msg *_Msg) error {
	if err := handler.StreamHandler.Handle(msg); err != nil {
		return err
	}

	if handler.isSyncWrite {
		if err := handler.file.Sync(); err != nil {
			return err
		}
	}

	return nil
}

func (handler *FileHandler) String() string {
	var b bytes.Buffer

	fmt.Fprintf(&b, "FileHandler{StreamHandler: %s, fname: %s, syncWrite: %t}",
		handler.StreamHandler.String(), handler.fileName, handler.isSyncWrite)

	return string(b.Bytes())
}

// Time interval of file rotates.
type RotateInterval uint32

const (
	HOUR      RotateInterval = 3600
	DAY       RotateInterval = 86400
	MINUTE    RotateInterval = 60
	HALF_HOUR RotateInterval = 1800
)

// A file handler which supports rotation by time interval.
type TimeRotateFileHandler struct {
	FileHandler
	interval    RotateInterval
	lastLogTime int64
	backupCount uint16
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
		interval:    interval,
		backupCount: backupCount}

	return rotateHandler, nil
}

func (handler *TimeRotateFileHandler) Handle(msg *_Msg) error {
	// Rotate file
	if handler.shouldRotate() {
		if err := handler.doRotate(); err != nil {
			return err
		}
	}

	return handler.FileHandler.Handle(msg)
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
	err = os.Rename(handler.fileName, handler.fileName+"_"+suffix)
	file, err := os.OpenFile(handler.fileName, _OPEN_FILE_FLAG, _OPEN_FILE_MODE)
	if err != nil {
		return err
	}
	handler.file = file
	handler.SetOutput(file)

	return nil
}

func (handler *TimeRotateFileHandler) String() string {
	var b bytes.Buffer

	fmt.Fprintf(&b, "TimeRotateFileHandler{FileHandler: %s, interval: %d, backupCount: %d}",
		handler.FileHandler.String(), handler.interval, handler.backupCount)

	return string(b.Bytes())
}

const (
	_        = iota
	KB int64 = 1 << (10 * iota)
	MB int64 = 1 << (10 * iota)
	GB int64 = 1 << (10 * iota)
)

// A file handler which supports rotation by file size.
type SizeRotateFileHandler struct {
	FileHandler
	maxBytes    int64
	curBytes    int64
	backupCount uint16
	suffix      int32
}

func NewSizeRotateFileHandler(
	fileName string,
	maxBytes int64,
	backupCount uint16) (*SizeRotateFileHandler, error) {
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
		maxBytes:    maxBytes,
		curBytes:    curBytes,
		backupCount: backupCount,
		suffix:      suffix}

	return rotateHandler, nil
}

func (handler *SizeRotateFileHandler) Handle(msg *_Msg) error {
	if msg.level < handler.level {
		return nil
	}

	if handler.formatter == nil {
		handler.formatter, _ = NewFormatter(defautlFormatStr)
	}

	logMsg := handler.formatter.Format(msg)
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

	if handler.isSyncWrite {
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

	maxSuffix, err := findMaxSuffix(handler.fileName)
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
			fmt.Fprintf(&sfn, "%s_%04d", handler.fileName, i+1)
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
		fmt.Fprintf(&nameBuf, "%s_%04d", handler.fileName, handler.suffix+1)
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

func (handler *SizeRotateFileHandler) String() string {
	var b bytes.Buffer

	fmt.Fprintf(&b, "SizeRotateFileHandler{FileHandler: %s, maxBytes: %d, backupCount: %d}",
		handler.FileHandler.String(), handler.maxBytes, handler.backupCount)

	return string(b.Bytes())
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
	handler.SetSyncMode(true)
	formater, _ := NewFormatter(defautlFormatStr)
	handler.SetFormatter(formater)
	return handler
}

func stdErrLog(msg string, err error) {
	buff := bytes.Buffer{}
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(&buff, "%s %s err: %s\n", ts, msg, err.Error())
	os.Stderr.WriteString(string(buff.Bytes()))
}

func init() {
	innerFuncNames = make(map[string]int)
	loggerPrefix := "gologging.(*Logger)."
	gologgingPrefix := "gologging."

	for _, lv := range []string{"Debug", "Info", "Error", "Exception", "Fatal"} {
		innerFuncNames[loggerPrefix+lv] = 1
		innerFuncNames[gologgingPrefix+lv] = 1
	}
}
