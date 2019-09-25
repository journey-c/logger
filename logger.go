package logger

import (
	"bytes"
	"fmt"
	"os"
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
	bufferName    string
}

func NewLoggerBuffer(bufferName string) *LoggerBuffer {
	return &LoggerBuffer{
		bufferContent: bytes.NewBuffer(make([]byte, 0, DefaultBufferSize)),
		bufferName:    bufferName,
	}
}

func (this *LoggerBuffer) WriteString(text string) {
	this.bufferLock.Lock()
	this.bufferContent.WriteString(text)
	this.bufferLock.Unlock()
}

type Logger struct {
	userBuffer bool

	logFile string
	backup  string

	fileLock map[string]*sync.Mutex
	file     map[string]*os.File

	bufferRWLock sync.RWMutex
	buffer       map[string]*LoggerBuffer
	bufferQueue  chan LoggerBuffer

	syncInterval time.Duration
}

func NewLogger(filename, backup, prefix string) (*Logger, error) {
	if strings.HasSuffix(filename, "/") {
		filename = filename[:len(filename)-1]
	}
	if _, err := os.Stat(filename); err != nil {
		err = os.MkdirAll(filename, 0777)
		if err != nil {
			return nil, err
		}
	}

	if strings.HasSuffix(backup, "/") {
		backup = backup[:len(backup)-1]
	}
	if _, err := os.Stat(backup); err != nil {
		err = os.MkdirAll(backup, 0777)
		if err != nil {
			return nil, err
		}
	}
	filelock := make(map[string]*sync.Mutex)
	logfile := make(map[string]*os.File)
	for _, level := range logLevel {
		switch level {
		case "debug":
			file, err := os.OpenFile(filename+"/"+prefix+"_debug.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
			if err != nil {
				return nil, err
			}
			logfile[level] = file
			filelock[level] = &sync.Mutex{}
		case "error":
			file, err := os.OpenFile(filename+"/"+prefix+"_error.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
			if err != nil {
				return nil, err
			}
			logfile[level] = file
			filelock[level] = &sync.Mutex{}
		case "warn":
			file, err := os.OpenFile(filename+"/"+prefix+"_warn.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
			if err != nil {
				return nil, err
			}
			logfile[level] = file
			filelock[level] = &sync.Mutex{}
		case "trace":
			file, err := os.OpenFile(filename+"/"+prefix+"_trace.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
			if err != nil {
				return nil, err
			}
			logfile[level] = file
			filelock[level] = &sync.Mutex{}
		}
	}

	return &Logger{
		userBuffer:   false,
		logFile:      filename,
		backup:       backup,
		fileLock:     filelock,
		file:         logfile,
		buffer:       make(map[string]*LoggerBuffer),
		bufferQueue:  make(chan LoggerBuffer),
		syncInterval: time.Second,
	}, nil
}

func (this *Logger) SetSysInterval(interval int) {
	this.syncInterval = time.Duration(interval)
}

func (this *Logger) EnableBuffer() {
	this.userBuffer = true
	go this.writeToQueue()
	go this.flushToDisk()
}

func (this *Logger) write(level, text string) {
	if this.userBuffer {
		this.bufferRWLock.RLock()
		if _, ok := this.buffer[level]; !ok {
			this.buffer[level] = NewLoggerBuffer(level)
		}
		this.bufferRWLock.RUnlock()
		this.buffer[level].WriteString(text)
	} else {
		// TODO 同步写现有逻辑太浪费fd
		this.fileLock[level].Lock()
		_, err := this.file[level].Write([]byte(text))
		if err != nil {
			this.fileLock[level].Unlock()
			fmt.Println(err.Error())
			return
		}
		_ = this.file[level].Sync()
		this.fileLock[level].Unlock()
	}
}

func (this *Logger) Debug(args ...interface{}) {
	this.write(DEBUG, format(args...))
}

func (this *Logger) Error(args ...interface{}) {
	this.write(ERROR, format(args...))
}

func (this *Logger) Warn(args ...interface{}) {
	this.write(WARN, format(args...))
}

func (this *Logger) Trace(args ...interface{}) {
	this.write(TRACE, format(args...))
}

func (this *Logger) writeToQueue() {
	ticker := time.NewTicker(this.syncInterval)
	defer ticker.Stop()
	for {
		<-ticker.C
		this.bufferRWLock.RLock()
		for name, buffer := range this.buffer {
			this.bufferQueue <- *buffer
			this.buffer[name] = NewLoggerBuffer(name)
		}
		this.bufferRWLock.RUnlock()
	}
}

func (this *Logger) flushToDisk() {
	for {
		select {
		case buffer := <-this.bufferQueue:
			this.fileLock[buffer.bufferName].Lock()
			_, err := this.file[buffer.bufferName].Write(buffer.bufferContent.Bytes())
			if err != nil {
				this.fileLock[buffer.bufferName].Lock()
				continue
			}
			_ = this.file[buffer.bufferName].Sync()
			this.fileLock[buffer.bufferName].Lock()
		}
	}
}

func format(args ...interface{}) string {
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
