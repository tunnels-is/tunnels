package core

import (
	"crypto/md5"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type uniqueLog struct {
	date time.Time
}

var (
	seenLogs = make(map[[md5.Size]byte]uniqueLog)
	logMutex = &sync.Mutex{}
)

func checkLogUniqueness(log *string) (shouldLog bool) {
	hash := md5.Sum([]byte(*log))
	logMutex.Lock()
	_, exists := seenLogs[hash]
	if !exists {
		seenLogs[hash] = uniqueLog{
			date: time.Now(),
		}
		logMutex.Unlock()
		return true
	}
	logMutex.Unlock()
	return false
}

func CleanUniqueLogMap(MONITOR chan int) {
	defer func() {
		RecoverAndLogToFile()
		time.Sleep(10 * time.Second)
		MONITOR <- 5
	}()

	logMutex.Lock()
	defer logMutex.Unlock()

	for i := range seenLogs {
		if time.Since(seenLogs[i].date).Seconds() > 8 {
			delete(seenLogs, i)
		}
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

func DEEP(Line ...interface{}) {
	if !C.DeepDebugLoggin {
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
	checkLogUniqueness(&x)

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
			log.Println("Log API queue full")
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
