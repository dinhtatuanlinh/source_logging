package slogging

import (
	"context"
	"github.com/rs/zerolog/log"
)

type Logger struct {
	requestID string
	apiID     string
}

const (
	XRequestID = "X-Request-ID"
	APIID      = "api_id"
)

func New(ctx context.Context) *Logger {
	requestID, _ := ctx.Value(XRequestID).(string)
	apiID, _ := ctx.Value(APIID).(string)
	return &Logger{
		requestID: requestID,
		apiID:     apiID,
	}
}

func (l *Logger) Info() {
	log.Info().
		Str(XRequestID, l.requestID).
		Str(APIID, l.apiID)
}

func (l *Logger) Error(err error) {
	log.Error().
		Str(XRequestID, l.requestID).
		Str(APIID, l.apiID).
		Err(err)
}
