package mylog

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

//Log 日志类
type Log struct {
	oinfo uint32

	m           sync.Mutex
	curFileSize int64
	maxFileSize int64
	file        *os.File
	fileDir     string
}

//Close 关闭日志
func (l *Log) Close() {
	l.m.Lock()
	l.curFileSize = 0
	l.maxFileSize = 0
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	l.m.Unlock()
}

func (l *Log) output(text string) {
	l.m.Lock()
	defer l.m.Unlock()
	if l.maxFileSize == 0 {
		return
	}
	if l.file == nil {
		filePath := l.fileDir + time.Now().Format("20060102150405")
		f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return
		}
		l.file = f
		finfo, err := f.Stat()
		if err != nil {
			l.curFileSize = l.maxFileSize
		} else {
			l.curFileSize = finfo.Size()
		}
	}
	n, err := l.file.WriteString(text)
	if err == nil {
		l.curFileSize += int64(n)
	}
	if l.curFileSize >= l.maxFileSize {
		l.file.Close()
		l.file = nil
		l.curFileSize = 0
	}
}

const minLogSize = 10 * 1024 * 1024

//SwitchInfo 切换信息输出
func (l *Log) SwitchInfo() {
	atomic.AddUint32(&(l.oinfo), 1)
}

//PrintlnInfo 输出信息
func (l *Log) PrintlnInfo(a ...interface{}) {
	if len(a) == 0 {
		return
	}
	val := atomic.LoadUint32(&(l.oinfo))
	if val%2 == 0 {
		return
	}
	text := fmt.Sprintln(a...)
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	l.output(fmt.Sprintf("INF %s %s:%d: %s", time.Now().Format("2006/01/02 15:04:05"), file, line, text))
}

//PrintfInfo 输出信息
func (l *Log) PrintfInfo(format string, a ...interface{}) {
	val := atomic.LoadUint32(&(l.oinfo))
	if val%2 == 0 {
		return
	}
	formatLen := len(format)
	if formatLen > 0 {
		if format[formatLen-1] != '\n' {
			format += "\n"
		}
	} else {
		format = "\n"
	}
	text := fmt.Sprintf(format, a...)
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	l.output(fmt.Sprintf("INF %s %s:%d: %s", time.Now().Format("2006/01/02 15:04:05"), file, line, text))
}

//PrintlnError 输出错误
func (l *Log) PrintlnError(a ...interface{}) {
	if len(a) == 0 {
		return
	}

	text := fmt.Sprintln(a...)
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	l.output(fmt.Sprintf("ERR %s %s:%d: %s", time.Now().Format("2006/01/02 15:04:05"), file, line, text))
}

//PrintfError 输出错误
func (l *Log) PrintfError(format string, a ...interface{}) {
	formatLen := len(format)
	if formatLen > 0 {
		if format[formatLen-1] != '\n' {
			format += "\n"
		}
	} else {
		format = "\n"
	}
	text := fmt.Sprintf(format, a...)
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "???"
		line = 0
	}
	l.output(fmt.Sprintf("ERR %s %s:%d: %s", time.Now().Format("2006/01/02 15:04:05"), file, line, text))
}

//NewLog 创建日志
func NewLog(maxSize int64, dir string) *Log {
	if maxSize < minLogSize {
		maxSize = minLogSize
	}
	var nl Log
	nl.maxFileSize = maxSize
	if len(dir) != 0 {
		if dir[len(dir)-1] == '/' {
			nl.fileDir = dir
		} else {
			nl.fileDir = dir + "/"
		}
	}
	return &nl
}
