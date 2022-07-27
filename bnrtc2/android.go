package bnrtc2

import (
	"fmt"

	"github.com/guabee/bnrtc/bnrtcv2/bootstrap"
	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/debug"
)

type Option func(config *conf.Config)

func WithHttpPort(port int) Option {
	return func(config *conf.Config) {
		config.Port = port
	}
}

func WithServicePort(port int) Option {
	return func(config *conf.Config) {
		config.ServicePort = port
	}
}

func WithDwebPort(port int) Option {
	return func(config *conf.Config) {
		if config.Services == nil {
			config.Services = make(map[string]conf.OptionMap)
		}

		options, ok := config.Services["dweb"]
		if !ok {
			config.Services["dweb"] = map[string]interface{}{"http_port": port}
		} else {
			options["http_port"] = port
		}
	}
}

func WithDebugPort(port int) Option {
	return func(config *conf.Config) {
		config.DebugPort = port
	}
}

func WithPeers(peers []string) Option {
	return func(config *conf.Config) {
		if len(peers) > 0 {
			peersWithPort := make([]string, len(peers))
			for i, v := range peers {
				peersWithPort[i] = fmt.Sprintf("%s:%d", v, config.ServicePort)
			}

			config.Peers = peersWithPort
		}
	}
}

func WithDirPath(dirPath string) Option {
	return func(conf *conf.Config) {
		conf.DirPath = dirPath
	}
}

var defaultPeers = []string{"35.200.84.105", "35.189.132.49", "34.84.204.111"}

func start(options ...Option) {
	config := conf.GetDefaultConfig()
	for _, opt := range options {
		opt(&config)
	}

	if !config.DisableDebug {
		go debug.StartServer(config.DebugPort)
	}
	for {
		live := make(chan string)
		go bootstrap.StartServer(config)
		<-live
	}
}

func Start(dirPath string) {
	start(WithDirPath(dirPath), WithPeers(defaultPeers))
}

func StartWithConfig(dirPath string, httpPort int, servicePort int, dwebPort int, debugPort int) {
	start(WithDirPath(dirPath), WithHttpPort(httpPort), WithServicePort(servicePort), WithDwebPort(dwebPort), WithDebugPort(debugPort), WithPeers(defaultPeers))
}
