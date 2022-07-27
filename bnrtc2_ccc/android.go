package bnrtc2

import (
	"fmt"

	"github.com/guabee/bnrtc/bnrtcv2/bootstrap"
	conf "github.com/guabee/bnrtc/bnrtcv2/config"
	"github.com/guabee/bnrtc/bnrtcv2/debug"
)

const (
	CCC_SIGNAL_SERVER_PORT = 19021
	CCC_HTTP_PORT          = 19120
	CCC_SERVICE_PORT       = 19121
	CCC_DEBUG_PORT         = 19122
	CCC_DWEB_PORT          = 19123
)

var defaultPeers = []string{"35.200.84.105", "35.189.132.49", "34.84.204.111"}

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
			config.Services["dweb"] = conf.OptionMap{"http_port": port}
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
		config.Peers = peers
	}
}

func WithDirPath(dirPath string) Option {
	return func(config *conf.Config) {
		config.DirPath = dirPath
	}
}

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
	StartWithConfig(dirPath, CCC_HTTP_PORT, CCC_SERVICE_PORT, CCC_DEBUG_PORT, CCC_DWEB_PORT)
}

func StartWithConfig(dirPath string, httpPort int, servicePort int, debugPort int, dwebPort int) {
	peersWithPort := make([]string, len(defaultPeers))
	for i, v := range defaultPeers {
		peersWithPort[i] = fmt.Sprintf("%s:%d", v, CCC_SIGNAL_SERVER_PORT)
	}

	start(WithDirPath(dirPath), WithHttpPort(httpPort), WithServicePort(servicePort), WithDebugPort(debugPort), WithDwebPort(dwebPort), WithPeers(peersWithPort))
}
