package main

import (
	"log"
	"os"

	"github.com/mhef/statera/cfg"
	"github.com/mhef/statera/lb"
)

// configFilePath define tha path of the file that will contain the application
// configuration.
const configFilePath = "/etc/statera/conf.json"

// loadConfig will open the config file and return the parsed config.
func loadConfig() (*cfg.Config, error) {
	r, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}
	cfg, err := cfg.Load(r)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func main() {
	log.Println("Statera started")
	defer log.Println("Statera stopped")
	defer func() {
		if r := recover(); r != nil {
			log.Panic(r)
		}
	}()

	lcfg, err := loadConfig()
	if err != nil {
		log.Fatalln("Could not load config file:", err)
		return
	}

	lb.Start(lcfg)
}
