package client

import (
	"crypto/md5"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"
)

type uniqueLog struct {
	date time.Time
}

func checkLogUniqueness(log *string) (shouldLog bool) {
	hash := md5.Sum([]byte(*log))
	hashStr := string(hash[:])
	_, exists := logRecordHash.Load(hashStr)
	if !exists {
		logRecordHash.Store(hashStr, true)
		return true
	}
	return false
}

func CleanUniqueLogMap() {
	defer func() {
		time.Sleep(10 * time.Second)
	}()
	defer RecoverAndLogToFile()
	logRecordHash.Clear()
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

func DEEP(Line ...any) {
	conf := CONFIG.Load()
	if !conf.DeepDebugLoggin {
		return
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf("%v", v)
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

func DEBUG(Line ...any) {
	conf := CONFIG.Load()
	if !conf.DebugLogging {
		return
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf("%v", v)
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

func ERROR(Line ...any) {
	conf := CONFIG.Load()
	if !conf.ErrorLogging {
		return
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf("%v", v)
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

func INFO(Line ...any) {
	conf := CONFIG.Load()
	if !conf.InfoLogging {
		return
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf("%v", v)
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

func StartLogQueueProcessor() {
	defer RecoverAndLogToFile()
	DEBUG("Starting the log processor")

	var line string
	for {
		line = <-LogQueue
		conf := CONFIG.Load()
		if conf.ConsoleLogging {
			fmt.Println(line)
		}

		if conf.ConsoleLogOnly {
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

func ErrorLog(err any, msgs ...any) {
	log.Println(TAG_ERROR+" || ", fmt.Sprint(msgs...), " >> system error: ", err)
}
