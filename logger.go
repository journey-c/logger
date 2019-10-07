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

	DefaultBufferSize   = 4 * KB
	DefaultLogSplitSize = 2 * GB

	DataFormat = "2006-01-02 15:04:05"

	DEBUG = "debug"
	WARN  = "warn"
	ERROR = "error"
	TRACE = "trace"
)

var logLevel = [4]string{"debug", "warn", "error", "trace"}

type Buffer struct {
	bufferLock    sync.Mutex
	bufferContent *bytes.Buffer
}

func NewLoggerBuffer() *Buffer {
	return &Buffer{
		bufferContent: bytes.NewBuffer(make([]byte, 0, DefaultBufferSize)),
	}
}

func (this *Buffer) WriteString(text string) {
	this.bufferLock.Lock()
	this.bufferContent.WriteString(text)
	this.bufferLock.Unlock()
}

type Mate struct {
	logId     int
	filename  string
	backupDir string
	file      *os.File

	DateStamp time.Time

	buffer      *Buffer
	bufferQueue chan *Buffer

	syncInterval time.Duration
}

func NewMate(filename, backupDir string) (*Mate, error) {
	dataStamp, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	loggerMate := &Mate{
		logId:        int(0),
		filename:     filename,
		backupDir:    backupDir,
		DateStamp:    dataStamp,
		buffer:       NewLoggerBuffer(),
		bufferQueue:  make(chan *Buffer, KB*4),
		syncInterval: time.Second,
	}

	err := loggerMate.CreateFile()
	if err != nil {
		return nil, err
	}

	go loggerMate.WriteToQueue()
	go loggerMate.FlushToDisk()

	return loggerMate, nil
}

func (this *Mate) SetSysInterval(interval int) {
	this.syncInterval = time.Duration(interval)
}

func (this *Mate) NeedSplit() (bool, bool) {
	fileStat, _ := this.file.Stat()
	nowData, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	if nowData.After(this.DateStamp) {
		fmt.Println(nowData.String() + "|" + this.DateStamp.String())
	}
	return fileStat.Size() >= DefaultLogSplitSize, nowData.After(this.DateStamp)
}

func (this *Mate) LogBackup() {
	var err error
	nowDataStr := time.Now().Format("2006-01-02")
	nowData, _ := time.Parse("2006-01-02", nowDataStr)
	_, err = os.Stat(this.backupDir)
	if err != nil {
		_ = os.Mkdir(this.backupDir, 0777)
	}
	_, err = os.Stat(filepath.Join(this.backupDir, nowDataStr))
	if err != nil {
		_ = os.Mkdir(filepath.Join(this.backupDir, nowDataStr), 0777)
	}
	if err := this.file.Close(); err != nil {
		return
	}
	for i := 0; i < this.logId; i++ {
		_ = os.Rename(this.filename+"."+strconv.Itoa(i), filepath.Join(filepath.Join(this.backupDir, nowDataStr),
			filepath.Base(this.filename)+"."+strconv.Itoa(i)))
	}
	_ = os.Rename(this.filename, filepath.Join(filepath.Join(this.backupDir, nowDataStr), filepath.Base(this.filename)+"."+strconv.Itoa(this.logId)))
	this.file = nil
	_ = this.CreateFile()

	this.logId = 0
	this.DateStamp = nowData
}

func (this *Mate) CreateFile() (err error) {
	this.file, err = os.OpenFile(this.filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0777)
	if err != nil {
		return
	}
	return nil
}

func (this *Mate) WriteToQueue() {
	ticker := time.NewTicker(this.syncInterval)
	defer ticker.Stop()
	for {
		<-ticker.C
		this.bufferQueue <- this.buffer
		this.buffer = NewLoggerBuffer()
	}
}

func (this *Mate) FlushToDisk() {
	for {
		select {
		case buffer := <-this.bufferQueue:
			var err error
			needSplit, needBackup := this.NeedSplit()
			if needBackup {
				this.LogBackup()
				continue
			}

			if needSplit {
				err = this.file.Close()
				if err != nil {
					continue
				}
				err = os.Rename(this.filename, this.filename+"."+strconv.Itoa(this.logId))
				if err != nil {
					this.logId++
					_ = this.CreateFile()
				}
			}

			if this.file == nil {
				_ = this.CreateFile()
			}
			_, err = this.file.Write(buffer.bufferContent.Bytes())
			if err != nil {
				continue
			}
			err = this.file.Sync()
			if err != nil {
				fmt.Println(err.Error())
			}
		}
	}
}

type Logger struct {
	mateLock sync.RWMutex
	Mates    map[string]*Mate
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

	tmpMates := make(map[string]*Mate)

	for _, level := range logLevel {
		switch level {
		case DEBUG:
			filename := filepath.Join(fileDir, module+"-"+GetInnerIp()+"-debug.log")
			loggerMate, err := NewMate(filename, filepath.Join(backupDir, module))
			if err != nil {
				return nil, err
			}
			tmpMates[DEBUG] = loggerMate
		case WARN:
			filename := filepath.Join(fileDir, module+"-"+GetInnerIp()+"-warn.log")
			loggerMate, err := NewMate(filename, filepath.Join(backupDir, module))
			if err != nil {
				return nil, err
			}
			tmpMates[WARN] = loggerMate
		case ERROR:
			filename := filepath.Join(fileDir, module+"-"+GetInnerIp()+"-error.log")
			loggerMate, err := NewMate(filename, filepath.Join(backupDir, module))
			if err != nil {
				return nil, err
			}
			tmpMates[ERROR] = loggerMate
		case TRACE:
			filename := filepath.Join(fileDir, module+"-"+GetInnerIp()+"-trace.log")
			loggerMate, err := NewMate(filename, filepath.Join(backupDir, module))
			if err != nil {
				return nil, err
			}
			tmpMates[TRACE] = loggerMate
		}
	}

	return &Logger{
		Mates: tmpMates,
	}, nil
}

func (this *Logger) write(level, text string) {
	this.mateLock.RLock()
	this.Mates[level].buffer.WriteString(text)
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
		if ipMask[0] != "127.0.0.1" {
			return ipMask[0]
		}
	}
	return ""
}
