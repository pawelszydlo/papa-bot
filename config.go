// Bot configuration handling

package main

import (
	"encoding/json"
	"io/ioutil"
)

type Configuration struct {
	Server      string
	Port        uint
	Name	    string
	User        string
	Channels	[]string
	Hellos	    []string
}


func LoadConfig(filename string) (Configuration, error) {
	var configuration Configuration

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return configuration, err
	}

	err = json.Unmarshal(file, &configuration)
	if err != nil {
		return configuration, err
	}

	return configuration, nil
}