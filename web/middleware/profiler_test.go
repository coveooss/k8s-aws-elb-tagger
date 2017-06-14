package middleware_test

import (
	"net/http"
	"testing"

	. "github.com/coveo/k8s-aws-elb-tagger/web/middleware"
)

var _ = Profiler

func TestProfilerImplementsHTTPHandler(t *testing.T) {
	// won't compile if SomeType does not implement SomeInterface
	var _ http.Handler = Profiler()
}
