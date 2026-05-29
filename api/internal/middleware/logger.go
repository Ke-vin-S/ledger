package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/Ke-vin-S/ledger/api/internal/logger"
)

// wrappedWriter captures status code and response size without changing behaviour.
type wrappedWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *wrappedWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *wrappedWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

// RequestLogger returns a structured HTTP logging middleware.
// Each request is logged with method, path, status, latency, request_id, and ip.
// 5xx → error, 4xx → warn, 2xx/3xx → info. /health is skipped.
// The per-request logger (with request_id pre-attached) is stored in the context
// so handlers can retrieve it via logger.FromContext(ctx).
func RequestLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			requestID := GetRequestID(r.Context())
			reqLog := log.With(zap.String("request_id", requestID))

			ww := &wrappedWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			next.ServeHTTP(ww, r.WithContext(logger.WithContext(r.Context(), reqLog)))

			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.status),
				zap.Duration("latency", time.Since(start)),
				zap.String("ip", r.RemoteAddr),
				zap.Int("bytes", ww.size),
			}

			switch {
			case ww.status >= 500:
				reqLog.Error("http request", fields...)
			case ww.status >= 400:
				reqLog.Warn("http request", fields...)
			default:
				reqLog.Info("http request", fields...)
			}
		})
	}
}
