/**
 *
 * @author  chosen0ne(louzhenlin86@126.com)
 * @date    2017-11-06 16:33:23
 */

package gologging

import (
	"testing"
)

func TestLoad(t *testing.T) {
	if err := Load("logger-sample.conf"); err != nil {
		t.Errorf("failed to Load 'logger-sample.conf', err: %s", err.Error())
	}

	elog := GetLogger("error")
	ilog := GetLogger("info")
	dlog := GetLogger("dev")

	elog.Error("test error")
	elog.Info("test info")
	elog.Debug("test debug")

	ilog.Error("info error")
	ilog.Info("info info")
	ilog.Debug("info debug")

	dlog.Info("test")
}
