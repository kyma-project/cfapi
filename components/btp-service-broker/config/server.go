package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type Server struct {
	InternalPort      int `yaml:"internalPort"`
	IdleTimeout       int `yaml:"idleTimeout"`
	ReadTimeout       int `yaml:"readTimeout"`
	ReadHeaderTimeout int `yaml:"readHeaderTimeout"`
	WriteTimeout      int `yaml:"writeTimeout"`

	RootNamespace string `yaml:"rootNamespace"`
}


func LoadServerConfig() (*Server, error) {
	configPath, found := os.LookupEnv("SERVER_CONFIG")
	if !found {
		return &Server{
			InternalPort: 8080,
			ReadTimeout: 900,
			WriteTimeout: 900,
			IdleTimeout: 900,
			ReadHeaderTimeout: 10,
			RootNamespace: "cfapi-system",
		}, nil
	}

	configFile, err := os.Open(filepath.Join(configPath, "server_config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to open server config file: %w", err)
	}
	defer configFile.Close()

	decoder := yaml.NewDecoder(configFile)
	conf := &Server{}
	if err = decoder.Decode(conf); err != nil {
		return nil, fmt.Errorf("failed decoding server config %q: %w", configPath, err)
	}

	return conf, nil
}
