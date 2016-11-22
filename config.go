package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path"
	"time"
)

type Host struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

type Config struct {
	AppName       string          `json:"app_name"`
	ConnType      string          `json:"conn_type"`
	ListenOn      Host            `json:"listen_on"` // 0.0.0.0 listen on all interfaces
	DestHosts     map[string]Host `json:"destination_hosts"`
	ChunkSize     int             `json:"chunk_size"`
	KeepAliveSecs time.Duration   `json:"keep_alive_secs"`
}

var Conf Config

func LoadConfig() io.Reader {
	fName := "config.json"
	fPath := path.Join(".", fName)
	file, err := os.Open(fPath)
	if err != nil {
		log.Fatalf("could not open: %v; ", err)
	}
	return file
}

func init() {
	log.SetFlags(log.Lshortfile)
	fileReader := LoadConfig()
	decoder := json.NewDecoder(fileReader)
	err := decoder.Decode(&Conf)
	if err != nil {
		log.Fatalf("could not decode json config: %v; ", err)
	}
	formatted, err := json.MarshalIndent(Conf, " ", "\t")
	if err != nil {
		log.Fatalf("could not format json config: %v; ", err)
	}
	log.Printf("\n%#s", string(formatted))
}
