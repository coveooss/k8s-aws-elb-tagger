package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/inconshreveable/log15"
)

// Recoverer is a middleware that recovers from panics, logs the panic (and a
// backtrace), and returns a HTTP 500 (Internal Server Error) status if
// possible.
//
// Originally taken from Goji middleware https://github.com/zenazn/goji/tree/master/web/middleware
func Recoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rvr := recover()
			if rvr == nil {
				return
			}

			logger := GetLogger(r)
			if logger == nil {
				logger = log15.Root()
			}

			logger.Error(fmt.Sprintf("PANIC: %v", rvr), "panic", rvr, "stack", string(debug.Stack()))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
