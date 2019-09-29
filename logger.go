package logger

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB

	DefaultBufferSize = 4 * KB

	DataFormat = "2006-02-02 15:04:05"

	DEBUG = "debug"
	WARN  = "warn"
	ERROR = "error"
	TRACE = "trace"
)

var logLevel = [4]string{"debug", "warn", "error", "trace"}

type LoggerBuffer struct {
	bufferLock    sync.Mutex
	bufferContent *bytes.Buffer
}

func NewLoggerBuffer() *LoggerBuffer {
	return &LoggerBuffer{
		bufferContent: bytes.NewBuffer(make([]byte, 0, DefaultBufferSize)),
	}
}

func (this *LoggerBuffer) WriteString(text string) {
	this.bufferLock.Lock()
	this.bufferContent.WriteString(text)
	this.bufferLock.Unlock()
}

type LoggerMate struct {
	backupDir string
	file      *os.File

	buffer      *LoggerBuffer
	bufferQueue chan *LoggerBuffer

	syncInterval time.Duration
}

func NewLoggerMate(filename, backupDir string) (*LoggerMate, error) {
	logFile, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
	if err != nil {
		return nil, err
	}
	loggerMate := &LoggerMate{
		backupDir:    backupDir,
		file:         logFile,
		buffer:       NewLoggerBuffer(),
		bufferQueue:  make(chan *LoggerBuffer, KB*4),
		syncInterval: time.Second,
	}
	go loggerMate.WriteToQueue()
	go loggerMate.FlushToDisk()

	return loggerMate, nil
}

func (this *LoggerMate) SetSysInterval(interval int) {
	this.syncInterval = time.Duration(interval)
}

func (this *LoggerMate) WriteToQueue() {
	ticker := time.NewTicker(this.syncInterval)
	defer ticker.Stop()
	for {
		<-ticker.C
		this.bufferQueue <- this.buffer
		this.buffer = NewLoggerBuffer()
	}
}

func (this *LoggerMate) FlushToDisk() {
	for {
		select {
		case buffer := <-this.bufferQueue:
			_, err := this.file.Write(buffer.bufferContent.Bytes())
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			_ = this.file.Sync()
		}
	}
}

type Logger struct {
	mateLock    sync.RWMutex
	loggerMates map[string]*LoggerMate
}

func NewLogger(fileDir, backupDir, module string) (*Logger, error) {
	if _, err := os.Stat(fileDir); err != nil {
		err = os.MkdirAll(fileDir, 0777)
		if err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(backupDir); err != nil {
		err = os.MkdirAll(backupDir, 0777)
		if err != nil {
			return nil, err
		}
	}

	tmpLoggerMates := make(map[string]*LoggerMate)

	for _, level := range logLevel {
		switch level {
		case DEBUG:
			filename := filepath.Join(fileDir, module+"-"+GetInnerIp()+"-debug.log")
			loggerMate, err := NewLoggerMate(filename, backupDir)
			if err != nil {
				return nil, err
			}
			tmpLoggerMates[DEBUG] = loggerMate
		case WARN:
			filename := filepath.Join(fileDir, module+"-"+GetInnerIp()+"-warn.log")
			loggerMate, err := NewLoggerMate(filename, backupDir)
			if err != nil {
				return nil, err
			}
			tmpLoggerMates[WARN] = loggerMate
		case ERROR:
			filename := filepath.Join(fileDir, module+"-"+GetInnerIp()+"-error.log")
			loggerMate, err := NewLoggerMate(filename, backupDir)
			if err != nil {
				return nil, err
			}
			tmpLoggerMates[ERROR] = loggerMate
		case TRACE:
			filename := filepath.Join(fileDir, module+"-"+GetInnerIp()+"-trace.log")
			loggerMate, err := NewLoggerMate(filename, backupDir)
			if err != nil {
				return nil, err
			}
			tmpLoggerMates[TRACE] = loggerMate
		}
	}

	return &Logger{
		loggerMates: tmpLoggerMates,
	}, nil
}

func (this *Logger) write(level, text string) {
	this.mateLock.RLock()
	this.loggerMates[level].buffer.WriteString(text)
	this.mateLock.RUnlock()
}

func (this *Logger) Debug(args ...interface{}) {
	this.write(DEBUG, Format(args...))
}

func (this *Logger) Error(args ...interface{}) {
	this.write(ERROR, Format(args...))
}

func (this *Logger) Warn(args ...interface{}) {
	this.write(WARN, Format(args...))
}

func (this *Logger) Trace(args ...interface{}) {
	this.write(TRACE, Format(args...))
}

func Format(args ...interface{}) string {
	content := time.Now().Format(DataFormat)
	for _, arg := range args {
		switch arg.(type) {
		case int:
			content = content + "|" + strconv.Itoa(arg.(int))
		case int64:
			content = content + "|" + strconv.FormatInt(arg.(int64), 10)
		case string:
			content = content + "|" + arg.(string)
		default:
			content = content + "|" + fmt.Sprintf("%v", arg)
		}
	}
	return content + "\n"
}

func GetInnerIp() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, add := range addrs {
		ipMask := strings.Split(add.String(), "/")
		if ipMask[0] != "127.0.0.1" && ipMask[0] != "24" {
			return ipMask[0]
		}
	}
	return ""
}
