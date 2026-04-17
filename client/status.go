package client

import (
	"errors"

	"github.com/codewandler/agentapis/internal/protocolcore"
)

type StatusError = protocolcore.StatusError

func StatusCodeOf(err error) (int, bool) {
	return protocolcore.StatusCodeOf(err)
}

func StatusErrorOf(err error) *StatusError {
	var statusErr *protocolcore.StatusError
	if errors.As(err, &statusErr) {
		return statusErr
	}
	var httpErr *protocolcore.HTTPError
	if errors.As(err, &httpErr) {
		return &protocolcore.StatusError{StatusCode: httpErr.StatusCode, Body: httpErr.Body}
	}
	return nil
}
