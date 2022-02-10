package log

import (
	"os"
	"sync"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/formatter"
	"github.com/projectdiscovery/gologger/levels"
)

var logger *gologger.Logger
var once sync.Once

const debugLogEnv = "DEBUG"

func init() {
	once.Do(func() {
		logger = gologger.DefaultLogger
		logger.SetFormatter(formatter.NewCLI(false))

		if os.Getenv("LOG_LEVEL") == debugLogEnv {
			logger.SetMaxLevel(levels.LevelDebug)
		} else {
			logger.SetMaxLevel(levels.LevelWarning)
		}
	})
}

func GetInstance() *gologger.Logger {
	return logger
}
