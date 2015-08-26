/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2014/10/31 16:25:18
 */

package gologging

import (
    "bytes"
    "time"
    "io"
    "fmt"
    "strconv"
    "path"
    "strings"
)

const (
    _DATE             = "date"
    _TIME             = "time"
    _DATETIME         = "datetime"
    _FUNCNAME         = "funcname"
    _FILENAME         = "filename"
    _FILEPATH         = "filepath"
    _LINENO           = "lineno"
    _LEVELNAME        = "levelname"
    _MESSAGE          = "message"
    _LOGGER_NAME      = "name"
    _ATTR_SEP         = '$'
    _ATTR_LEFT        = '{'
    _ATTR_RIGHT       = '}'
    _STR_PLACE_HOLDER = "%s"
    _NEWLINE          = '\n'
    _FUNC_DEPTH       = 3
)

var (
    defautlFormatStr    string
    attrs               map[string]interface{}
)

type Formatter struct {
    formatStr       string
    valFunc         []interface{}
}

func NewFormatter(formatStr string) (*Formatter, error) {
    valFuncs, fmtStr, err := parseFmtStr(formatStr)
    if err != nil {
        return nil, err
    }

    formatter := &Formatter{fmtStr, valFuncs}

    return formatter, nil
}

//func (format *Formatter) Format(loggerName string, level Level, message []byte) []byte {
func (format *Formatter) Format(msg *_Msg) []byte {
    attrs := make([]interface{}, 0)
    for _, attrFunc := range format.valFunc {
        fn := attrFunc.(func (*_Msg) string)
        attrs = append(attrs, fn(msg))
    }

    outputBuf := bytes.Buffer{}
    outputBuf.Write([]byte(""))
    fmt.Fprintf(&outputBuf, format.formatStr, attrs...)

    return outputBuf.Bytes()
}

// Format: '${datetime} - ${filename}:${lineno} - ${levelname} - ${message}'
// Parse the format string, to generate a format string for printf and a func to
// evaluate the attribute value
func parseFmtStr(fmtStr string) ([]interface{}, string, error) {
    fmtBuf := bytes.Buffer{}
    valFuncs := make([]interface{}, 0)

    buf := bytes.NewBufferString(fmtStr)
    // Find a attribute each round, attribute is included in '${}'
    for {
        // Find '$'
        rbytes, err := buf.ReadBytes(_ATTR_SEP)
        if err == io.EOF {
            if len(rbytes) != 0 {
                wlen, err := fmtBuf.Write(rbytes)
                if err != nil || wlen != len(rbytes) {
                    return nil, "", err
                }
            }

            break;
        } else if err != nil {
            return nil, "", err
        }

        // Find '{'
        nbyte, err := buf.ReadByte()
        if err != nil {
            return nil, "", err
        }

        // Output data just read except '$'
        wlen, err := fmtBuf.Write(rbytes[:len(rbytes)-1])
        if err != nil || wlen != len(rbytes) - 1 {
            return nil, "", err
        }

        if nbyte != _ATTR_LEFT {
            if err = fmtBuf.WriteByte(nbyte); err != nil {
                return nil, "", err
            }
            continue
        }

        // Found left part of attr, '${'
        // Find '}'
        rbytes, err = buf.ReadBytes(_ATTR_RIGHT)
        if err != nil {
            return nil, "", err
        }

        // Found attr
        attr := string(rbytes[:len(rbytes)-1])
        valFunc, ok := attrs[attr]
        if !ok {
            // Not supported attribute
            fmt.Println("not support attr:", attr, valFunc, "\n", attrs)
            return nil, "", err
        } else {
            valFuncs = append(valFuncs, valFunc)
            fmtBuf.WriteString(_STR_PLACE_HOLDER)
        }
    }
    fmtBuf.WriteByte(_NEWLINE)

    return valFuncs, string(fmtBuf.Bytes()), nil
}

func getDate(_ *_Msg) string {
    return time.Now().Format("2006-01-02")
}

func getTime(_ *_Msg) string {
    return time.Now().Format("15:04:05")
}

func getDateTime(_ *_Msg) string {
    return time.Now().Format("2006-01-02 15:04:05")
}

func getFilePath(msg *_Msg) string {
    return msg.fileName
}

func getFileName(msg *_Msg) string {
    return path.Base(msg.fileName)
}

func getLineNo(msg *_Msg) string {
    return strconv.Itoa(msg.lineNo)
}

func getFuncName(msg *_Msg) string {
    parts := strings.Split(msg.funcName, "/")
    if len(parts) <= 0 {
        return ""
    }
    return parts[len(parts) - 1]
}

func getMessage(msg *_Msg) string {
    return string(msg.message)
}

func getLevelName(msg *_Msg) string {
    return msg.level.Name()
}

func getLoggerName(msg *_Msg) string {
    return msg.loggerName
}

func init() {
    attrs = make(map[string]interface{})
    attrs[_DATE] = getDate
    attrs[_TIME] = getTime
    attrs[_DATETIME] = getDateTime
    attrs[_FILENAME] = getFileName
    attrs[_FILEPATH] = getFilePath
    attrs[_FUNCNAME] = getFuncName
    attrs[_LINENO] = getLineNo
    attrs[_MESSAGE] = getMessage
    attrs[_LEVELNAME] = getLevelName
    attrs[_LOGGER_NAME] = getLoggerName

    defautlFormatStr = "${datetime} [${levelname}]-${filename}:${lineno}:${funcname}-${name}-${message}"
}
