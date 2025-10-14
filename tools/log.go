package tools

import "github.com/go-logr/logr"

func LogAndReturn(logger logr.Logger, err error) error {
	logger.Error(err, err.Error())
	return err
}
