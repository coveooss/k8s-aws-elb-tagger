package middleware

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
)

var requestLoggerKey = &contextKey{"Logger"}

func RequestLogger(logger log15.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Add request id to logger if it is here (it should)
			reqID := GetReqID(r)
			l := logger
			if reqID != "" {
				l = logger.New("reqID", reqID)
			}

			// TODO: wrapResponseWriter to rereadit
			wr := &wrappedResponse{0, 0, w}
			t1 := time.Now()
			defer func() {
				t2 := time.Now()
				t2.Sub(t1)

				l.Info(fmt.Sprintf("//%s%s %s", r.Host, r.RequestURI, r.Proto),
					"method", r.Method,
					"path", r.URL.String(),
					"duration", t2.Sub(t1),
					"size", wr.bytes,
					"status", wr.status)
				// write the log
			}()

			r = r.WithContext(context.WithValue(r.Context(), requestLoggerKey, l))
			next.ServeHTTP(wr, r)
		}

		return http.HandlerFunc(fn)
	}
}

func GetLogger(r *http.Request) log15.Logger {
	logger, _ := r.Context().Value(requestLoggerKey).(log15.Logger)
	return logger
}

type wrappedResponse struct {
	status int
	bytes  int
	http.ResponseWriter
}

var _ http.ResponseWriter = &wrappedResponse{}
var _ http.CloseNotifier = &wrappedResponse{}
var _ http.Flusher = &wrappedResponse{}
var _ http.Pusher = &wrappedResponse{}
var _ http.Hijacker = &wrappedResponse{}
var _ io.ReaderFrom = &wrappedResponse{}

func (w *wrappedResponse) WriteHeader(i int) {
	w.status = i
	w.ResponseWriter.WriteHeader(i)
}

func (w *wrappedResponse) Write(b []byte) (int, error) {
	w.bytes = binary.Size(b)
	return w.ResponseWriter.Write(b)
}
func (w *wrappedResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.WithStack(errors.New("does not implement http.Hijack"))
}

func (w *wrappedResponse) Flush() {
	w.ResponseWriter.(http.Flusher).Flush()
}

func (w *wrappedResponse) CloseNotify() <-chan bool {
	return w.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (w *wrappedResponse) Push(target string, opts *http.PushOptions) error {
	return w.ResponseWriter.(http.Pusher).Push(target, opts)
}

func (w *wrappedResponse) ReadFrom(r io.Reader) (int64, error) {
	return w.ResponseWriter.(io.ReaderFrom).ReadFrom(r)
}
