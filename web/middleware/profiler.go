package middleware

import (
	"expvar"
	"net/http"
	"net/http/pprof"

	"github.com/pressly/chi"
)

// Profiler is a convenient subrouter used for mounting net/http/pprof. ie.
//
//  func MyService() http.Handler {
//    r := chi.NewRouter()
//    // ..middlewares
//    r.Mount("/debug", middleware.Profiler())
//    // ..routes
//    return r
//  }
func Profiler() http.Handler {
	r := chi.NewRouter()

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.RequestURI+"/pprof/", 301)
	})
	r.HandleFunc("/pprof", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.RequestURI+"/", 301)
	})

	r.HandleFunc("/pprof/", pprof.Index)
	r.HandleFunc("/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/pprof/profile", pprof.Profile)
	r.HandleFunc("/pprof/symbol", pprof.Symbol)
	r.Handle("/pprof/block", pprof.Handler("block"))
	r.Handle("/pprof/heap", pprof.Handler("heap"))
	r.Handle("/pprof/goroutine", pprof.Handler("goroutine"))
	r.Handle("/pprof/threadcreate", pprof.Handler("threadcreate"))
	//r.HandleFunc("/vars", expVars)
	r.Mount("/vars", expvar.Handler())

	return r
}
