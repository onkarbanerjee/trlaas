package logging

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
	fbLog "trlaas/flatbuf/LogInterface"
	"trlaas/types"
	"trlaas/utility"

	fb "github.com/google/flatbuffers/go"
	"github.com/nats-io/go-nats"
)

type LogLevel int

const (
	LogLevelAll LogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarning
	LogLevelError
	LogLevelException
)

type LogConfiguration struct {
	StandardOutputOnly bool   // true to print o/p on console. by dafault false. No console print
	DateFormat         string //  date format. Check time package for info
	TimeFormat         string // Time format. Check time package for info
	TimeLocation       string // Time and Date Location. time package ex: Asia/Kolkata
	PrintException     bool   // If true exception type will be printed. else not
	PrintError         bool   // If true error type will be printed. else not
	PrintWarning       bool   // If true Warning type will be printed. else not
	PrintInfo          bool   // If true Info  type will be printed. else not
	PrintDebug         bool   // If true Debug type will be printed. else not
}

var (
	//NatsConn nats connection string
	NatsConn    *nats.Conn
	err         error
	LogConf     LogConfiguration
	ContainerID string
)

func init() {
	LogConf.StandardOutputOnly, _ = strconv.ParseBool(os.Getenv("LOGGING_STDOUT_LOCAL"))
}

// getLogLevelEnum takes log level as string and returns LogLevel enum
func getLogLevelEnum(level string) LogLevel {
	levelMap := map[string]LogLevel{
		"ALL":       LogLevelAll,
		"DEBUG":     LogLevelDebug,
		"INFO":      LogLevelInfo,
		"WARNING":   LogLevelWarning,
		"ERROR":     LogLevelError,
		"EXCEPTION": LogLevelException,
	}
	if _, ok := levelMap[level]; !ok {
		return LogLevelAll
	}
	return levelMap[level]
}

func InitializeLogInfo() {
	LogConf.DateFormat = "YY-MM-DD"
	LogConf.TimeFormat = "SS-MM-HH-MIC"
	LogConf.TimeLocation = ""

	setLogLevel()
}

func setLogLevel() {
	if types.TpaasConfigObj.TpaasConfig.App.LogLevel == "" {
		types.TpaasConfigObj.TpaasConfig.App.LogLevel = "INFO"
	}

	logLevelEnum := getLogLevelEnum(types.TpaasConfigObj.TpaasConfig.App.LogLevel)
	if logLevelEnum <= LogLevelException {
		LogConf.PrintException = true
	} else {
		LogConf.PrintException = false
	}

	if logLevelEnum <= LogLevelError {
		LogConf.PrintError = true
	} else {
		LogConf.PrintError = false
	}

	if logLevelEnum <= LogLevelWarning {
		LogConf.PrintWarning = true
	} else {
		LogConf.PrintWarning = false
	}

	if logLevelEnum <= LogLevelInfo {
		LogConf.PrintInfo = true
	} else {
		LogConf.PrintInfo = false
	}

	if logLevelEnum <= LogLevelDebug {
		LogConf.PrintDebug = true
	} else {
		LogConf.PrintDebug = false
	}
}

func (lc *LogConfiguration) Exception(arg ...interface{}) {
	if !lc.PrintException {
		return
	}
	lc.createLogEntry("EXCEPTION", arg)
	os.Exit(1)
}

func (lc *LogConfiguration) Error(arg ...interface{}) {
	if !LogConf.PrintError {
		return
	}
	lc.createLogEntry("ERROR", arg)
}

func (lc *LogConfiguration) Warning(arg ...interface{}) {
	if !LogConf.PrintWarning {
		return
	}
	lc.createLogEntry("WARNING", arg)
}

func (lc *LogConfiguration) Info(arg ...interface{}) {
	if !LogConf.PrintInfo {
		return
	}
	lc.createLogEntry("INFO", arg)
}

func (lc *LogConfiguration) Debug(arg ...interface{}) {
	if !LogConf.PrintDebug {
		return
	}
	lc.createLogEntry("DEBUG", arg)
}

func (lc *LogConfiguration) createLogEntry(level string, arg ...interface{}) {
	ts, _ := utility.Get(lc.DateFormat, lc.TimeFormat, lc.TimeLocation)
	if lc.StandardOutputOnly {
		fmt.Println("["+level+"] : ", ts, "->", arg)
		return
	}
	LogForwarder(fmt.Sprintf("["+level+"] : "+ts+" -> %v\n", arg...))
}

//NatsConnection nats connection
func NatsConnection() {
	var urls = flag.String("s", nats.DefaultURL, "The nats server URLs (separated by comma)")

	// Connect Options.
	opts := []nats.Option{nats.Name("NATS TRL Publisher")}
	for {
		NatsConn, err = nats.Connect(*urls, opts...)
		if err != nil {
			log.Println("no nats connection", err)
			log.Println("Retrying NATS")
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
	log.Println("nats connection is successful")
}

//LogForwarder forwrds the log to nats
func LogForwarder(logs string) {
	msgs := natsMsgConvertion(logs)
	err := NatsConn.Publish("LOG", msgs)
	if err != nil {
		log.Println("error while publishing message to nats", err)
	}
	NatsConn.Flush()
}

//natsMsgConvertion msg convertions to flatbuf
func natsMsgConvertion(logs string) []byte {
	builder := fb.NewBuilder(0)

	cntID := builder.CreateString(ContainerID)
	cntName := builder.CreateString("tpaas")
	fbPayload := builder.CreateString(logs)

	fbLog.LogMessageStart(builder)
	fbLog.LogMessageAddContainerId(builder, cntID)
	fbLog.LogMessageAddContainerName(builder, cntName)
	fbLog.LogMessageAddPayload(builder, fbPayload)
	logMsg := fbLog.LogMessageEnd(builder)

	builder.Finish(logMsg)

	return builder.FinishedBytes()
}
