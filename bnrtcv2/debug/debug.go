package debug

import (
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"

	"github.com/guabee/bnrtc/bnrtcv2/log"
)

func handleLogStatus(wr http.ResponseWriter, r *http.Request) {
	loggersInfo := log.GetLoggersInfo()
	bytes, err := json.Marshal(loggersInfo)
	if err != nil {
		_, _ = wr.Write([]byte(err.Error()))
	} else {
		_, _ = wr.Write(bytes)
	}
}

func handleLogLevel(wr http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	name := query.Get("name")
	level := query.Get("level")

	log.SetLogLevel(name, log.Level(level))
	_, _ = wr.Write([]byte("ok"))
}

func handleLogMemory(wr http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	enable := query.Get("enable")
	size := query.Get("size")
	//string to bool
	enableBool, _ := strconv.ParseBool(enable)
	sizeInt, _ := strconv.ParseInt(size, 10, 64)
	log.ToggleLogToBuffer(enableBool, sizeInt)
	_, _ = wr.Write([]byte("ok"))
}

func handleLogDump(wr http.ResponseWriter, r *http.Request) {
	bytes := log.LogDump()
	if bytes == nil {
		_, _ = wr.Write([]byte("nil"))
	} else {
		_, _ = wr.Write(bytes)
	}
}

func handleHelp(wr http.ResponseWriter, r *http.Request) {
	for _, handler := range pathHandlers {
		str := fmt.Sprintf("%s %v\n", handler.Path, handler.Params)
		_, _ = wr.Write([]byte(str))
	}
}

type debugHandlerInfo struct {
	Path   string
	Handle func(http.ResponseWriter, *http.Request)
	Params []string
}

var pathHandlers = []debugHandlerInfo{
	{"/logstatus", handleLogStatus, nil},
	{"/loglevel", handleLogLevel, []string{"name", "level"}},
	{"/logmemory", handleLogMemory, []string{"enable", "size"}},
	{"/logdump", handleLogDump, nil},
}

func init() {
	for _, handler := range pathHandlers {
		http.HandleFunc(handler.Path, handler.Handle)
	}
	http.HandleFunc("/help", handleHelp)
}

type DebugOption func()

func WithDefaultLogLevel(level string) DebugOption {
	return func() {
		if level != "" {
			log.SetDefaultLevel(log.Level(level))
		}
	}
}

func WithDebugLogModules(modules []string) DebugOption {
	return func() {
		for _, moduleName := range modules {
			allSubModule := moduleName
			if moduleName != "*" && moduleName != "all" {
				allSubModule = fmt.Sprintf("%s*", moduleName)
			}
			log.SetLogLevel(allSubModule, log.DebugLevel)
		}
	}
}

func WithDebugLogToBuffer(size int64) DebugOption {
	return func() {
		if size <= 0 {
			size = 5 * 1024 * 1024
		}
		log.ToggleLogToBuffer(true, size)
	}
}

func WithDebugLogToFile(filename string) DebugOption {
	return func() {
		if filename != "" {
			fd, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				fmt.Printf("open file %s for logging failed: %s.\n", filename, err.Error())
				return
			}
			log.SetLogOut(fd)
		}
	}
}

func StartServer(port int, options ...DebugOption) {
	for _, option := range options {
		option()
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		fmt.Printf("debug server start failed %s\n", err)
	}
}
