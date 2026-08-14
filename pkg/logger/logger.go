package logger

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

var (
	mu     sync.Mutex
	logger *log.Logger
)

func Init(baseDir string) error {
	mu.Lock()
	defer mu.Unlock()

	logger = log.New(os.Stdout, "", 0)
	logUnsafe("INFO", "LOGGER", "Logger initialized. Outputting to console (stdout)")
	return nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	logger = nil
}

func logUnsafe(level, tag, msg string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	formatted := fmt.Sprintf("[%s] [%s] [%s] %s", timestamp, level, tag, msg)
	if logger != nil {
		logger.Println(formatted)
	} else {
		log.Println(formatted)
	}
}

func Infof(tag, format string, v ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	logUnsafe("INFO", tag, fmt.Sprintf(format, v...))
}

func Debugf(tag, format string, v ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	logUnsafe("DEBUG", tag, fmt.Sprintf(format, v...))
}

func Warnf(tag, format string, v ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	logUnsafe("WARN", tag, fmt.Sprintf(format, v...))
}

func Errorf(tag, format string, v ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	logUnsafe("ERROR", tag, fmt.Sprintf(format, v...))
}
