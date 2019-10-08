# logger
a golang async logger

### run a logger test
```
go test -v github.com/journey-c/logger
```
### example
```cassandraql
package main

import (
	"fmt"
	"time"

	"github.com/journey-c/logger"
)

func main() {
	testLogger, err := logger.NewLogger("/tmp/log", "tmp/log/backup", "room")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	for {
		time.Sleep(time.Second)
		go testLogger.Trace("Trace", "Test", time.Now().String())
		go testLogger.Warn("Warn", "Test", time.Now().String())
		go testLogger.Debug("Debug", "Test", time.Now().String())
		go testLogger.Error("Error", "Test", time.Now().String())
	}
}
```
