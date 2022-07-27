package config

import (
	"flag"
	"fmt"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/spf13/cast"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	BuildName string = ""
)

type BatchConfig struct {
	StartHttpPort       int           `json:"startHttpPort"`
	StartServicePort    int           `json:"startServicePort"`
	StartBnrtc1StunPort int           `json:"startBnrtc1StunPort"`
	StartBnrtc1TurnPort int           `json:"startBnrtc1TurnPort"`
	PeersNum            int           `json:"peersNum"`
	BatchNum            int           `json:"batchNum"`
	Addresses           []string      `json:"addresses"`
	DisableTurnServer   bool          `json:"disableTurnServer"`
	DisableStunServer   bool          `json:"disableStunServer"`
	DefaultTurnUrls     []string      `json:"defaultTurnUrls"`
	DefaultStunUrls     []string      `json:"defaultStunUrls"`
	BpType              int           `json:"bpType"`
	WaitTime            time.Duration `json:"waitTime"`
}

type DHTMemoryStoreOptions struct {
	StorePath  string `json:"storePath"`
	UseDbStore bool   `json:"useDbStore"`
}

type OptionMap map[string]interface{}

func (o OptionMap) GetInt(key string) (i int) {
	if v, ok := o[key]; ok {
		i = cast.ToInt(v)
	}
	return
}

func (o OptionMap) GetString(key string) (s string) {
	if v, ok := o[key]; ok {
		s = cast.ToString(v)
	}
	return
}

func (o OptionMap) GetStringSlice(key string) (s []string) {
	if v, ok := o[key]; ok {
		s = cast.ToStringSlice(v)
	}
	return
}

func (o OptionMap) GetDuration(key string) (d time.Duration) {
	if v, ok := o[key]; ok {
		d = cast.ToDuration(v)
	}
	return
}

func (o OptionMap) GetBool(key string, defaultValue bool) (b bool) {
	b = defaultValue
	if v, ok := o[key]; ok {
		b = cast.ToBool(v)
	}
	return
}

type Config struct {
	Name                  string                 `json:"name"`
	Host                  string                 `json:"host"`
	ExternalIp            string                 `json:"externalIp"`
	Port                  int                    `json:"port"`
	ServicePort           int                    `json:"servicePort"`
	DebugPort             int                    `json:"debugPort"`
	Bnrtc1StunPort        int                    `json:"bnrtc1StunPort"`
	Bnrtc1TurnPort        int                    `json:"bnrtc1TurnPort"`
	ForceTopNetwork       bool                   `json:"forceTopNetwork"`
	Peers                 []string               `json:"peers"`
	DevId                 string                 `json:"devid"`
	Addresses             []string               `json:"addresses"`
	SignalConnectionLimit int                    `json:"signalConnectionLimit"`
	ConnectionLimit       int                    `json:"connectionLimit"`
	ConnectionExpired     time.Duration          `json:"connectionExpired"`
	DebugModules          []string               `json:"debugModules"`
	LogLevel              string                 `json:"logLevel"`
	LogToBuffer           bool                   `json:"logToBuffer"`
	DisableTurnServer     bool                   `json:"disableTurnServer"`
	DisableStunServer     bool                   `json:"disableStunServer"`
	DefaultTurnUrls       []string               `json:"defaultTurnUrls"`
	DefaultStunUrls       []string               `json:"defaultStunUrls"`
	DisableIpv6           bool                   `json:"disableIpv6"`
	DisableLan            bool                   `json:"disableLan"`
	DisableDebug          bool                   `json:"disableDebug"`
	BpType                int                    `json:"bpType"`
	BackPressureLimit     int                    `json:"backPressureLimit"`
	EnableUdpAck          bool                   `json:"enableUdpAck"`
	UdpAckTimeout         time.Duration          `json:"udpAckTimeout"`
	Batch                 *BatchConfig           `json:"batch"`
	Fork                  bool                   `json:"fork"`
	DHTMemoryStoreOptions *DHTMemoryStoreOptions `json:"dhtMemoryStoreOptions"`
	DirPath               string                 `json:"dirPath"`
	Services              map[string]OptionMap   `json:"services"`
	ServiceManager        OptionMap              `json:"serviceManager"`
	LogFile               string                 `json:"logFile"`
}

func init() {
	pflag.String("name", "", "name of the server")
	pflag.String("host", "", "external http service address")
	pflag.Uint("port", 0, "local http service port")
	pflag.Uint("servicePort", 0, "external http service port")
	pflag.Bool("forceTopNetwork", false, "force topnework device")
	pflag.String("addresses", "", "binding chain addresses")
	pflag.String("devid", "", "device ID")
	pflag.String("peers", "", "signal server host list")
	pflag.String("configFile", "", "use custom config file")
	pflag.String("config", "", "use custom config file")
	pflag.String("debugModules", "", "open debug modules")
	pflag.Bool("version", false, "version info")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		fmt.Println("bind flags error:", err)
	}
}

func GetDefaultConfig() Config {
	return Config{
		Name:                  "Bnrtc2",
		Host:                  "0.0.0.0",
		Port:                  19020,
		ServicePort:           19021,
		DebugPort:             19022,
		Bnrtc1StunPort:        19001,
		Bnrtc1TurnPort:        19001,
		ForceTopNetwork:       false,
		SignalConnectionLimit: 20000,
		ConnectionLimit:       1000,
		BackPressureLimit:     5000,
		ConnectionExpired:     30 * 60 * time.Second,
		DisableTurnServer:     true,
		DisableStunServer:     true,
		DisableIpv6:           true,
		DisableLan:            false,
		EnableUdpAck:          true,
		UdpAckTimeout:         10 * time.Second,
		Fork:                  false,
	}
}

func BuildConfig(batch *BatchConfig, peers []string, address string, offset int) Config {
	config := Config{
		Name:                  address,
		Host:                  "0.0.0.0",
		Port:                  batch.StartHttpPort + offset,
		ServicePort:           batch.StartServicePort + offset,
		ForceTopNetwork:       false,
		SignalConnectionLimit: 20000,
		ConnectionLimit:       1000,
		BackPressureLimit:     5000,
		Addresses:             []string{address},
		Peers:                 peers,
		DisableTurnServer:     batch.DisableTurnServer,
		DisableStunServer:     batch.DisableStunServer,
		DefaultTurnUrls:       batch.DefaultTurnUrls,
		DefaultStunUrls:       batch.DefaultStunUrls,
		BpType:                batch.BpType,
		DisableDebug:          true,
	}

	if batch.PeersNum > 0 && offset < len(peers) {
		config.ForceTopNetwork = true
	}
	config.DevId = deviceid.NewDeviceId(config.ForceTopNetwork, nil, nil).String()
	return config
}

func ParseConfig() Config {
	// viper.AutomaticEnv()
	pflag.Parse()
	viper.SetConfigName("config")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./conf")

	file := viper.GetString("configFile")
	if file == "" {
		file = viper.GetString("config")
	}
	if file != "" {
		viper.SetConfigFile(file)
	}
	config := GetDefaultConfig()

	err := viper.ReadInConfig()
	if err == nil {
		err = viper.Unmarshal(&config)
		if err != nil {
			fmt.Println("read config file error:", err)
		}
	}
	return config
}

func VersionOnly() bool {
	return viper.GetBool("version")
}

func GetString(key string) string {
	return viper.GetString(key)
}

func GetBool(key string) bool {
	return viper.GetBool(key)
}

func GetInt(key string) int {
	return viper.GetInt(key)
}

func GetDuration(key string) time.Duration {
	return viper.GetDuration(key)
}

func GetValue(key string) interface{} {
	return viper.Get(key)
}
