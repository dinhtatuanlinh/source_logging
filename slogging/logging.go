package slogging

import (
	"context"
	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"runtime"
	"time"
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

type Options struct {
	Service     string
	Environment string
	Pretty      bool // keep false in prod for JSON
	Level       string
	WithCaller  bool
	SampleEvery int
	// New:
	FilePath    string    // if set, logs go to this file with rotation
	MaxSizeMB   int       // rotate after size (e.g., 100)
	MaxBackups  int       // keep N old files
	MaxAgeDays  int       // days to keep
	Compress    bool      // gzip old logs
	AlsoStdout  bool      // tee to stdout as well (useful with system collectors)
	ExtraWriter io.Writer // optional: any additional writer (e.g., socket)
}

type ctxKey string

const (
	ctxLoggerKey       ctxKey = "logger"
	ctxReqIDKey        ctxKey = "request_id"
	ctxOperatorNameKey ctxKey = "operator_name"
	ctxRoleKey         ctxKey = "role"
	ctxTraceIDKey      ctxKey = "trace_id"
	ctxApiIDKey        ctxKey = "api_id"
	ctxIPAddressKey    ctxKey = "ip_address"
)

// Init sets the global logger (log.Logger) and base fields.
func Init(opt Options) {
	zerolog.TimeFieldFormat = time.RFC3339

	lvl, err := zerolog.ParseLevel(opt.Level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	// Build the output writer
	var w io.Writer
	if opt.FilePath != "" {
		r := &lumberjack.Logger{
			Filename:   opt.FilePath,
			MaxSize:    max(1, opt.MaxSizeMB),
			MaxBackups: max(0, opt.MaxBackups),
			MaxAge:     max(0, opt.MaxAgeDays),
			Compress:   opt.Compress,
		}
		switch {
		case opt.AlsoStdout && opt.ExtraWriter != nil:
			w = zerolog.MultiLevelWriter(r, os.Stdout, opt.ExtraWriter)
		case opt.AlsoStdout:
			w = zerolog.MultiLevelWriter(r, os.Stdout)
		case opt.ExtraWriter != nil:
			w = zerolog.MultiLevelWriter(r, opt.ExtraWriter)
		default:
			w = r
		}
	} else {
		// No file path -> default to stdout (good for containers)
		w = os.Stdout
	}

	// Pretty should stay false in prod; pretty = human output (not JSON)
	var base zerolog.Logger
	if opt.Pretty {
		base = zerolog.New(zerolog.ConsoleWriter{Out: w}).With().Timestamp().Logger()
	} else {
		base = zerolog.New(w).With().Timestamp().Logger()
	}

	if opt.SampleEvery > 1 {
		base = base.Sample(&zerolog.BasicSampler{N: uint32(opt.SampleEvery)})
	}

	fields := base.With().
		Str("service", opt.Service).
		Str("env", opt.Environment)

	if opt.WithCaller {
		zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
			fn := runtime.FuncForPC(pc)
			if fn == nil {
				return file + ":" + itoa(line)
			}
			return fn.Name() + " " + file + ":" + itoa(line)
		}
		fields = fields.Caller()
	}

	log.Logger = fields.Logger()
}

// With returns a child logger with more fields (without touching global).
func With(kv ...any) zerolog.Logger {
	return log.Logger.With().Fields(kvToMap(kv...)).Logger()
}

// IntoContext stores a logger into ctx (merging given fields) using zerolog's native context.
func IntoContext(ctx context.Context, kv ...any) context.Context {
	fields := kvToMap(kv...)
	if base := log.Ctx(ctx); base != nil && base.GetLevel() != zerolog.Disabled {
		ll := base.With().Fields(fields).Logger()
		return ll.WithContext(ctx) // ✅ store under zerolog's key
	}
	ll := log.Logger.With().Fields(fields).Logger()
	return ll.WithContext(ctx) // ✅ store under zerolog's key
}

// From extracts the logger from ctx; falls back to global.
// From extracts the logger from ctx; falls back to global.
func From(ctx context.Context) *zerolog.Logger {
	if ctx == nil {
		return &log.Logger
	}
	if l := log.Ctx(ctx); l != nil { // ← zerolog-native context lookup
		return l
	}
	// (optional) compat path if you still have old code that used your custom key:
	if l, ok := ctx.Value(ctxLoggerKey).(*zerolog.Logger); ok && l != nil {
		return l
	}
	return &log.Logger
}

// Helpers to set/read common IDs on context
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return IntoContext(context.WithValue(ctx, ctxReqIDKey, reqID), "request_id", reqID)
}
func WithAPIID(ctx context.Context, apiID string) context.Context {
	return IntoContext(context.WithValue(ctx, ctxApiIDKey, apiID), "api_id", apiID)
}
func WithOperatorName(ctx context.Context, operatorID string) context.Context {
	return IntoContext(context.WithValue(ctx, ctxOperatorNameKey, operatorID), "operator_name", operatorID)
}
func WithRole(ctx context.Context, role any) context.Context {
	return IntoContext(context.WithValue(ctx, ctxRoleKey, role), "role", role)
}
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return IntoContext(context.WithValue(ctx, ctxTraceIDKey, traceID), "trace_id", traceID)
}
func WithIPAddress(ctx context.Context, ipAddress string) context.Context {
	return IntoContext(context.WithValue(ctx, ctxIPAddressKey, ipAddress), "ip_address", ipAddress)
}

func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxReqIDKey).(string); ok {
		return v
	}
	return ""
}

func OperatorID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxOperatorNameKey).(string); ok {
		return v
	}
	return ""
}

func Role(ctx context.Context) any {
	return ctx.Value(ctxRoleKey)
}

func TraceID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxTraceIDKey).(string); ok {
		return v
	}
	return ""
}

// --- internals ---

func kvToMap(kv ...any) map[string]any {
	m := make(map[string]any)
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		m[k] = kv[i+1]
	}
	return m
}

// tiny, allocation-free itoa for caller format
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
