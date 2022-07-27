package main

import (
	"fmt"

	"github.com/guabee/bnrtc/bnrtcv2/bootstrap"
	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/debug"
	"github.com/guabee/bnrtc/util"
)

var (
	BuildVersion = "v0.0.0-build.0"
	CommitID     = "Local"
	BuildTime    = "2006-01-02 15:04:05"
	BuildName    = "Lion"
)

func main() {
	isChild := util.IsChildProcess()
	if !isChild { // daemon it
		fmt.Printf("%s %s %s %s\n", BuildVersion, BuildName, CommitID, BuildTime)
		_ = conf.ParseConfig() // check config
		if !conf.VersionOnly() {
			_ = util.ForkRunLoop()
		}
		return
	}
	conf.BuildName = BuildName
	config := conf.ParseConfig()
	go bootstrap.StartServer(config)

	if !config.DisableDebug {
		go debug.StartServer(
			config.DebugPort, debug.WithDefaultLogLevel(config.LogLevel),
			debug.WithDebugLogModules(config.DebugModules),
			debug.WithDebugLogToFile(config.LogFile),
		)
	}
	c := make(chan struct{})
	<-c
}
