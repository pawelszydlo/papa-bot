// Bot configuration handling

package main

import (
	"encoding/json"
	"io/ioutil"
)

type Configuration struct {
	Server      string
	Name	    string
	User        string
	OwnerPassword string
	Channels	[]string
	Hellos	    []string
	HellosAfterKick	    []string
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

	if configuration.OwnerPassword  == "" {  // don't allow empty passwords
		lerror.Fatal("You must set OwnerPassword in your config.")
	}

	return configuration, nil
}