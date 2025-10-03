package helpers

import (
	"fmt"
	"os"
)

func MustGetEnv(envVar string) string {
	envVarValue, ok := os.LookupEnv(envVar)
	if !ok {
		panic(fmt.Sprintf("env var %s is not set", envVar))
	}

	return envVarValue
}
