package routing

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyma-project/cfapi/osbapi"
)

type Response struct {
	httpStatus int
	body       interface{}
	headers    map[string][]string
}

func NewResponse(httpStatus int) *Response {
	return &Response{
		httpStatus: httpStatus,
		headers:    map[string][]string{},
	}
}

func (r *Response) WithHeader(key, value string) *Response {
	r.headers[key] = append(r.headers[key], value)
	return r
}

func (r *Response) WithBody(body interface{}) *Response {
	r.body = body
	return r
}

type Handler func(r *http.Request) (*Response, error)

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := logr.FromContextOrDiscard(r.Context())

	handlerResponse, err := h(r)
	if err != nil {
		logger.Info("handler returned error", "reason", err)
		presentError(logger, w, err)
		return
	}

	if err := handlerResponse.writeTo(w); err != nil {
		logger.Error(err, "failed to write result to the HTTP response", "handlerResponse", handlerResponse, "method", r.Method, "URL", r.URL)
	}
}

func presentError(logger logr.Logger, w http.ResponseWriter, err error) {
	var sbError osbapi.ServiceBrokerError
	if errors.As(err, &sbError) {
		responseBody := map[string]any{
			"error":       sbError.ErrorCode(),
			"description": sbError.Description(),
		}
		if sbError.InstanceUsable() != nil {
			responseBody["instance_usable"] = *sbError.InstanceUsable()
		}
		if sbError.UpdateRepeatable() != nil {
			responseBody["update_repeatable"] = *sbError.UpdateRepeatable()
		}

		writeErr := NewResponse(sbError.HttpStatus()).WithBody(responseBody).writeTo(w)

		if writeErr != nil {
			logger.Error(writeErr, "failed to write error to the HTTP response")
		}

		return
	}

	presentError(logger, w, osbapi.NewUnknownError(err))
}

func (response *Response) writeTo(w http.ResponseWriter) error {
	for header, headerValues := range response.headers {
		for _, value := range headerValues {
			w.Header().Add(header, value)
		}
	}

	if response.body == nil {
		w.WriteHeader(response.httpStatus)
		return nil
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.httpStatus)

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(response.body)
	if err != nil {
		return fmt.Errorf("failed to encode and write response: %w", err)
	}

	return nil
}
