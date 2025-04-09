package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ServiceManager struct {
	ClientId     string `json:"clientid"`
	ClientSecret string `json:"clientsecret"`
	SmUrl        string `json:"sm_url"`
	TokenUrl     string `json:"tokenurl"`
}

func LoadServiceManagerClientConfig() (*ServiceManager, error) {
	configPath, found := os.LookupEnv("SM_CLIENT_CONFIG")
	if !found {
		return nil, errors.New("SM_CLIENT_CONFIG env var not set")
	}

	items, err := os.ReadDir(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config dir %q: %w", configPath, err)
	}

	configMap := map[string]string{}
	for _, item := range items {
		fileName := item.Name()
		if item.IsDir() || strings.HasPrefix(fileName, ".") {
			continue
		}

		var fileBytes []byte
		fileBytes, err = os.ReadFile(filepath.Join(configPath, fileName))
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}

		configMap[fileName] = string(fileBytes)
	}

	configBytes, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal service manager client config: %w", err)
	}

	smClientConfig := &ServiceManager{}
	err = json.Unmarshal(configBytes, smClientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal service manager client config: %w", err)
	}

	return smClientConfig, nil
}
