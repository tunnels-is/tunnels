package client

import (
	"crypto/md5"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	wgdevice "golang.zx2c4.com/wireguard/device"
)

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
	defer RecoverAndLog()
	logRecordHash.Clear()
}

func GET_FUNC(skip int) string {
	pc := make([]uintptr, 10)
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
	state := STATE.Load()
	if !state.Debug {
		if !conf.DebugLogging {
			return
		}
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf("%v ", v)
	}

	select {
	case LogQueue <- fmt.Sprintf(
		"%s || DEEP || %s || %s",
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
	state := STATE.Load()
	if !state.Debug {
		if !conf.DebugLogging {
			return
		}
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf("%v ", v)
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
	state := STATE.Load()
	if !state.Debug {
		if !conf.DebugLogging {
			return
		}
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf("%v ", v)
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

// SECURITY logs a security-relevant warning that must NOT be suppressed by the
// debug-logging flag (unlike ERROR/INFO). It writes straight to stdout and, if
// possible, also enqueues to the in-app log stream. Use for things the operator
// must see regardless of log level — e.g. TLS verification disabled, firewall
// off.
func SECURITY(Line ...any) {
	x := ""
	for _, v := range Line {
		x += fmt.Sprintf("%v ", v)
	}
	msg := fmt.Sprintf("%s || SECURITY || %s || %s",
		time.Now().Format("01-02 15:04:05"), GET_FUNC(3), x)
	log.Println(msg)
	select {
	case LogQueue <- msg:
	default:
	}
}

func INFO(Line ...any) {
	conf := CONFIG.Load()
	state := STATE.Load()
	if !state.Debug {
		if !conf.DebugLogging {
			return
		}
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

func ROUTINE(Line ...any) {
	conf := CONFIG.Load()
	state := STATE.Load()
	if !state.Debug {
		if !conf.DebugLogging {
			return
		}
	}

	x := ""
	for _, v := range Line {
		x += fmt.Sprintf("%v ", v)
	}

	select {
	case LogQueue <- fmt.Sprintf(
		"%s || ROUTINE || %s || %s",
		time.Now().Format("01-02 15:04:05"),
		GET_FUNC(3),
		fmt.Sprint(x),
	):
	default:
		ErrorLog(false, "COULD NOT PLACE LOG IN THE LOG QUEUE")
	}
}

func StartLogQueueProcessor() {
	defer RecoverAndLog()
	DEBUG("Starting the log processor")

	var line string
	for {
		line = <-LogQueue
		conf := CONFIG.Load()
		state := STATE.Load()
		if conf.ConsoleLogging || state.Debug {
			fmt.Println(line)
		}

		if conf.ConsoleLogOnly {
			continue
		}

		select {
		case APILogQueue <- line:
		default:
			log.Println("Log queue full, draining first half of the queue")
			for range len(APILogQueue) / 2 {
				select {
				case <-APILogQueue:
				default:
				}
			}
		}

		PollLogMu.Lock()
		PollLogBuf = append(PollLogBuf, line)
		if len(PollLogBuf) > 5000 {
			PollLogBuf = PollLogBuf[len(PollLogBuf)-5000:]
		}
		PollLogMu.Unlock()

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

// wgInfoPatterns are substrings of wireguard-go Verbosef format strings that
// represent meaningful state-change events worth surfacing at INFO level.
// Anything not matched is treated as DEBUG (per-packet, per-routine noise).
var wgInfoPatterns = []string{
	"Interface up requested",
	"Interface down requested",
	"Interface state was",
	"Device closing",
	"Device closed",
	"MTU updated",
	"UDP bind has been updated",
	"- Starting",
	"- Stopping",
	"Handshake did not complete after %d attempts, giving up",
	"Removing all keys, since we haven't received a new one",
	"Retrying handshake because we stopped hearing back",
}

func isWGInfoFormat(format string) bool {
	for _, p := range wgInfoPatterns {
		if strings.Contains(format, p) {
			return true
		}
	}
	return false
}

func wgLog(level, format string, args ...any) {
	conf := CONFIG.Load()
	state := STATE.Load()
	if !state.Debug && !conf.DebugLogging {
		return
	}

	msg := fmt.Sprintf(format, args...)
	select {
	case LogQueue <- fmt.Sprintf(
		"%s || %s || wg-client || %s",
		time.Now().Format("01-02 15:04:05"),
		level,
		msg,
	):
	default:
		ErrorLog(false, "COULD NOT PLACE LOG IN THE LOG QUEUE")
	}
}

// NewWGLogger returns a wireguard-go *device.Logger that routes through the
// app's LogQueue, inheriting the app's debug gating, console/API/file
// fan-out, and timestamp formatting. Errorf maps to ERROR; Verbosef is
// classified into INFO (state changes) or DEBUG (per-packet/routine noise)
// via isWGInfoFormat.
func NewWGLogger() *wgdevice.Logger {
	return &wgdevice.Logger{
		Verbosef: func(format string, args ...any) {
			if isWGInfoFormat(format) {
				wgLog("INFO ", format, args...)
			} else {
				wgLog("DEBUG", format, args...)
			}
		},
		Errorf: func(format string, args ...any) {
			wgLog("ERROR", format, args...)
		},
	}
}
