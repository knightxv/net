package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type UserConfigReaderWriter struct {
	FileName string
}

type UserConfig struct {
	Peers     []string `json:"peers,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	Favorites []string `json:"favorites,omitempty"`
}

func (rw UserConfigReaderWriter) Read() (config UserConfig) {
	byteValue, err := ioutil.ReadFile(rw.FileName)
	if err == nil {
		err = json.Unmarshal(byteValue, &config)
		if err != nil {
			fmt.Println("Error reading user config:", err)
		}
	}
	return
}

func (rw UserConfigReaderWriter) Write(config UserConfig) {
	jsonValue, _ := json.Marshal(config)
	err := ioutil.WriteFile(rw.FileName, jsonValue, 0644)
	if err != nil {
		fmt.Println("Error writing user config:", err)
	}
}
