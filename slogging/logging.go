package slogging

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Logger struct {
	requestID string
	apiID     string
	operator  string
}

const (
	XRequestID = "X-Request-ID"
	APIID      = "api_id"
	XOperator  = "x-operator"
)

func New(ctx context.Context) *Logger {
	requestID, _ := ctx.Value(XRequestID).(string)
	apiID, _ := ctx.Value(APIID).(string)
	operator, _ := ctx.Value(XOperator).(string)
	return &Logger{
		requestID: requestID,
		apiID:     apiID,
		operator:  operator,
	}
}

func (l *Logger) Info() *zerolog.Event {
	return log.Info().
		Str(XRequestID, l.requestID).
		Str(APIID, l.apiID).
		Str(XOperator, l.operator)
}

func (l *Logger) Error(err error) *zerolog.Event {
	return log.Error().
		Str(XRequestID, l.requestID).
		Str(APIID, l.apiID).
		Str(XOperator, l.operator).
		Err(err)
}
