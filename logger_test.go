package logger

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestASyncLogger(t *testing.T) {
	var wait sync.WaitGroup
	logger, err := NewLogger("/tmp/log", "/tmp/log/backup", "async-127.0.0.1-")
	defer func() {
		logger = nil
	}()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	for i := 0; i < 10000; i++ {
		wait.Add(1)
		go func(item int) {
			logger.Debug(item, "debug")
			logger.Warn(item, "warn")
			logger.Error(item, "error")
			logger.Trace(item, "trace")
			wait.Done()
		}(i)
	}
	wait.Wait()
	time.Sleep(time.Second * 5)
}