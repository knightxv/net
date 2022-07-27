package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/bootstrap"
	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/debug"
)

var (
	BuildVersion = "v0.0.0-build.0"
	CommitID     = "Local"
	BuildTime    = "2006-01-02 15:04:05"
	BuildName    = "Lion"
)

var (
	validPorts   []int = []int{}
	invalidPorts []int = []int{}
)

func isTcpPortAvailable(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	defer ln.Close()
	return true
}

func isUdpPortAvailable(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.ListenPacket("udp", addr)
	if err != nil {
		return false
	}
	defer ln.Close()
	return true
}

func startServer(c conf.Config) {
	if !isTcpPortAvailable("127.0.0.1", c.Port) || !isTcpPortAvailable("0.0.0.0", c.Port) {
		invalidPorts = append(invalidPorts, c.Port)
		return
	}

	if !isTcpPortAvailable("127.0.0.1", c.ServicePort) || !isTcpPortAvailable("0.0.0.0", c.ServicePort) {
		invalidPorts = append(invalidPorts, c.Port)
		return
	}
	if !isUdpPortAvailable("127.0.0.1", c.ServicePort) || !isUdpPortAvailable("0.0.0.0", c.ServicePort) {
		invalidPorts = append(invalidPorts, c.Port)
		return
	}
	validPorts = append(validPorts, c.Port)
	bootstrap.StartServer(c)
}

func main() {
	fmt.Printf("%s %s %s %s\n", BuildVersion, BuildName, CommitID, BuildTime)
	config := conf.ParseConfig()
	if conf.VersionOnly() {
		os.Exit(0)
	}
	if config.Batch == nil {
		fmt.Println("for batch test only")
		os.Exit(1)
	}

	var peers []string
	if config.Batch.PeersNum > 0 {
		for i := 0; i < config.Batch.PeersNum; i++ {
			peers = append(peers, fmt.Sprintf("127.0.0.1:%d", config.Batch.StartServicePort+i))
		}
	} else {
		peers = config.Peers
	}
	if len(config.Batch.Addresses) > 0 {
		for i, address := range config.Batch.Addresses {
			newConfig := conf.BuildConfig(config.Batch, peers, address, i)
			go startServer(newConfig)
			if config.Batch.WaitTime > 0 {
				time.Sleep(config.Batch.WaitTime)
			}
		}
	} else if config.Batch.BatchNum > 0 {
		for i := 0; i < config.Batch.BatchNum; i++ {
			newConfig := conf.BuildConfig(config.Batch, peers, strconv.Itoa(i), i)
			go startServer(newConfig)
			if config.Batch.WaitTime > 0 {
				time.Sleep(config.Batch.WaitTime)
			}
		}
	}
	if len(validPorts) > 0 {
		data, err := json.Marshal(validPorts)
		if err == nil {
			err = ioutil.WriteFile("./portInfo.txt", data, 0666)
		}
		if err != nil {
			fmt.Println("write validPorts file error", err)
		}
	}

	if len(invalidPorts) > 0 {
		data, err := json.Marshal(invalidPorts)
		if err == nil {
			err = ioutil.WriteFile("./wrongPortInfo.txt", data, 0666)
		}
		if err != nil {
			fmt.Println("write invalidPorts file error", err)
		}
	}

	if !config.DisableDebug {
		go debug.StartServer(config.DebugPort, debug.WithDefaultLogLevel(config.LogLevel), debug.WithDebugLogModules(config.DebugModules))
	}
	c := make(chan struct{})
	<-c
}
