package observability

import (
	"context"
	"sync/atomic"
)

// ErrorCaptureFunc receives an error with its request context for external
// reporting.
type ErrorCaptureFunc func(ctx context.Context, err error)

// errorSink is the process-wide error reporting hook. It is unset (CaptureError
// is a no-op) until live error tracking is decided on: no Sentry account or
// DSN exists yet, and the SDK is deliberately not vendored before that
// decision. When it happens, wire it in internal/app newRouter next to
// chimw.Recoverer: initialize the SDK from a SENTRY_DSN environment variable
// (mirroring how cfg reads other secrets) and call SetErrorSink with an
// adapter that forwards to the SDK's capture call. Call sites then report via
// CaptureError — candidates are the Recoverer's recovered panics and handler
// 5xx paths.
//
//nolint:gochecknoglobals // process-wide sink, same pattern as the cipher default in pkg/security
var errorSink atomic.Pointer[ErrorCaptureFunc]

// SetErrorSink installs the process-wide error sink. Passing nil restores the
// no-op default.
func SetErrorSink(sink ErrorCaptureFunc) {
	if sink == nil {
		errorSink.Store(nil)

		return
	}

	errorSink.Store(&sink)
}

// CaptureError reports err to the configured sink. With no sink installed it
// is a no-op, so call sites can be wired today and start reporting as soon as
// a backend is configured.
func CaptureError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	if sink := errorSink.Load(); sink != nil && *sink != nil {
		(*sink)(ctx, err)
	}
}
