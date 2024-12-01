package core

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

func (l *LoggerInterface) Print(message string) {
	if !PRODUCTION {
		log.Println(message)
	}
}

func (l *LoggerInterface) Trace(message string) {
	if !PRODUCTION {
		log.Println(message)
	}
}

func (l *LoggerInterface) Debug(message string) {
	if !PRODUCTION {
		log.Println(message)
	}
}

func (l *LoggerInterface) Info(message string) {
	if !PRODUCTION {
		log.Println(message)
	}
}

func (l *LoggerInterface) Warning(message string) {
	if !PRODUCTION {
		log.Println(message)
	}
}

func (l *LoggerInterface) Error(message string) {
	if !PRODUCTION {
		log.Println(message)
	}
}

func (l *LoggerInterface) Fatal(message string) {
	if !PRODUCTION {
		log.Println(message)
	}
}

func InitPacketTraceFile() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	GLOBAL_STATE.TracePath = GLOBAL_STATE.BasePath
	GLOBAL_STATE.TraceFileName = GLOBAL_STATE.TracePath + time.Now().Format("15-04-05") + ".trace.log"

	var err error
	TraceFile, err = os.Create(GLOBAL_STATE.TraceFileName)
	if err != nil {
		ERROR("Unable to create trace file: ", err)
		return
	}

	err = os.Chmod(GLOBAL_STATE.TraceFileName, 0o777)
	if err != nil {
		ERROR("Unable to change mode of trace file: ", err)
		return
	}

	DEBUG("New trace created: ", TraceFile.Name())
	GLOBAL_STATE.TraceFileInitialized = true
}

func InitLogfile() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	GLOBAL_STATE.LogPath = GLOBAL_STATE.BasePath
	GLOBAL_STATE.LogFileName = GLOBAL_STATE.LogPath + time.Now().Format("2006-01-02-15-04-05") + ".log"

	var err error
	LogFile, err = os.Create(GLOBAL_STATE.LogFileName)
	if err != nil {
		ERROR("Unable to create log file: ", err)
		return
	}

	err = os.Chmod(GLOBAL_STATE.LogFileName, 0o777)
	if err != nil {
		ERROR("Unable to change ownership of log file: ", err)
		return
	}

	DEBUG("New log file created: ", LogFile.Name())
	GLOBAL_STATE.LogFileInitialized = true
}

func GET_FUNC(skip int) string {
	pc := make([]uintptr, 10) // at least 1 entry needed
	runtime.Callers(skip, pc)
	f := runtime.FuncForPC(pc[0])
	name := f.Name()
	sn := strings.Split(name, ".")
	if sn[len(sn)-1] == "func1" {
		return sn[len(sn)-2]
	}
	return sn[len(sn)-1]
}

func DEBUG(Line ...interface{}) {
	if !C.DebugLogging {
		return
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf(" %v", v)
	}

	select {
	case LogQueue <- fmt.Sprintf(
		"%s || DEBUG || %s || %s",
		time.Now().Format("01-02 15:04:05"),
		GET_FUNC(3),
		fmt.Sprint(x),
	):
	default:
		ErrorLog(false, "COULD NOT PLACE LOG IN THE LOG QUEUE")
	}
}

func ERROR(Line ...interface{}) {
	if !C.ErrorLogging {
		return
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf(" %v", v)
	}

	select {
	case LogQueue <- fmt.Sprintf(
		"%s || ERROR || %s || %s",
		time.Now().Format("01-02 15:04:05"),
		GET_FUNC(3),
		fmt.Sprint(x),
	):
	default:
		ErrorLog(false, "COULD NOT PLACE LOG IN THE LOG QUEUE")
	}
}

func INFO(Line ...interface{}) {
	if !C.InfoLogging {
		return
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf(" %v", v)
	}

	select {
	case LogQueue <- fmt.Sprintf(
		"%s || INFO  || %s || %s",
		time.Now().Format("01-02 15:04:05"),
		GET_FUNC(3),
		fmt.Sprint(x),
	):
	default:
		ErrorLog(false, "COULD NOT PLACE LOG IN THE LOG QUEUE")
	}
}

func StartLogQueueProcessor(MONITOR chan int) {
	defer func() {
		MONITOR <- 1
	}()
	defer RecoverAndLogToFile()
	DEBUG("Logging module started")

	var line string
	for {
		line = <-LogQueue
		if C.ConsoleLogging {
			fmt.Println(line)
		}

		if C.ConsoleLogOnly {
			continue
		}

		select {
		case APILogQueue <- line:
		default:
			APILogQueue = nil
			APILogQueue = make(chan string, 1000)
			fmt.Println("Log API queue full")
		}

		if LogFile != nil {
			_, err := LogFile.WriteString(line + "\n")
			if err != nil {
				ErrorLog(err)
			}
		}
	}
}

func ErrorLog(err interface{}, msgs ...interface{}) {
	log.Println(TAG_ERROR+" || ", fmt.Sprint(msgs...), " >> system error: ", err)
}
