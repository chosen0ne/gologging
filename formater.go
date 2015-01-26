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
    "runtime"
    "strconv"
    "strings"
)

const (
    _DATE             = "date"
    _TIME             = "time"
    _DATETIME         = "datetime"
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
)

var (
    defautlFormatStr    string
    attrs               map[string]interface{}
)

type Formatter struct {
    formatStr       string
    valFunc         []interface{}
    msgIdx          int
    levelIdx        int
    loggerNameIdx   int
}


func NewFormatter(formatStr string) *Formatter {
    valFuncs, fmtStr, err := parseFmtStr(formatStr)
    if err != nil {
        fmt.Println("failed to parseFmtStr")
        return nil
    }

    msgIdx, levelIdx, loggerNameIdx := -1, -1, -1
    // Find msg index
    for idx, val := range valFuncs {
        switch val {
        case _MESSAGE:
            msgIdx = idx
        case _LEVELNAME:
            levelIdx = idx
        case _LOGGER_NAME:
            loggerNameIdx = idx
        }
    }

    if msgIdx == -1 {
        return nil
    }

    formatter := &Formatter{fmtStr, valFuncs, msgIdx, levelIdx, loggerNameIdx}

    return formatter
}

func (format *Formatter) Format(loggerName string, level Level, message []byte) []byte {
    attrs := make([]interface{}, 0)
    for idx, attrFunc := range format.valFunc {
        if idx == format.msgIdx {
            attrs = append(attrs, string(message))
        } else if idx == format.levelIdx {
            attrs = append(attrs, level.Name())
        } else if idx == format.loggerNameIdx {
            attrs = append(attrs, loggerName)
        } else {
            fn := attrFunc.(func () string)
            attrs = append(attrs, fn())
        }
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

func getDate() string {
    return time.Now().Format("2006-01-02")
}

func getTime() string {
    return time.Now().Format("15:04:05")
}

func getDateTime() string {
    return time.Now().Format("2006-01-02 15:04:05")
}

func getFilePath() string {
    _, file, _, ok := runtime.Caller(1)
    if !ok {
        file = "unkown"
    }
    return file
}

func getFileName() string {
    path := getFilePath()
    parts := strings.Split(path, "/")
    return parts[len(parts) - 1]
}

func getLineNo() string {
    _, _, line, ok := runtime.Caller(1)
    if !ok {
        line = -1
    }
    return strconv.Itoa(line)
}

func init() {
    attrs = make(map[string]interface{})
    attrs[_DATE] = getDate
    attrs[_TIME] = getTime
    attrs[_DATETIME] = getDateTime
    attrs[_FILENAME] = getFileName
    attrs[_FILEPATH] = getFilePath
    attrs[_LINENO] = getLineNo
    attrs[_MESSAGE] = _MESSAGE
    attrs[_LEVELNAME] = _LEVELNAME
    attrs[_LOGGER_NAME] = _LOGGER_NAME

    defautlFormatStr = "${datetime} ${filename}:${lineno}-[${levelname}]-${name}-${message}"
}


