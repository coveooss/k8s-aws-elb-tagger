package main

import (
	"expvar"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
)

const (
	// DefaultPort is the port the service is listening by default
	DefaultPort = "3000"

	// ServiceAnnotationTagPrefix is the prefix to put on k8s elb service annotations
	// to add tags to the AWS ELBs.
	//
	// ex:
	//  k8s service annotation: aws-tag/owner="John Doe"
	//  AWS tag will be: owner="John Doe"
	ServiceAnnotationTagPrefix = "aws-tag/"
	// ServiceAnnotationTagKeyPrefix is the annotation prefix for the key of the AWS tag
	// applied to the ELB associated to the service
	//
	// ex:
	//  k8s service annotation
	//    - aws-tag-key/1 = owner
	//    - aws-tag-value/1 = John Doe
	ServiceAnnotationTagKeyPrefix = "aws-tag-key/"
	// ServiceAnnotationTagValuePrefix is the annotation prefix for the value of the AWS tag
	// applied to the ELB associated to the service
	ServiceAnnotationTagValuePrefix = "aws-tag-value/"
)

func main() {
	dry := flag.Bool("dry", false, "Do not apply changes to the ELBs")
	flag.Parse()

	logger := log15.Root()
	logger.Info("Server Initializing")

	// Dependency Injection and initialization
	prometheusRegistry := prometheus.NewRegistry()

	prometheusRegistry.MustRegister(prometheus.NewProcessCollector(os.Getpid(), ""))
	prometheusRegistry.MustRegister(prometheus.NewGoCollector())

	// AWS initialization
	sess := session.Must(session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true)))
	elbAPI := elb.New(sess)

	// Kubernetes initialization
	config, err := kubernetesConfig()
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// Web Server initialization
	m := http.NewServeMux()
	m.HandleFunc("/", homeHandler)
	m.HandleFunc("/healthz", healthHandler)
	m.Handle("/metrics", promhttp.HandlerFor(prometheusRegistry, promhttp.HandlerOpts{}))
	m.HandleFunc("/debug/pprof/", pprof.Index)
	m.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	m.HandleFunc("/debug/pprof/profile", pprof.Profile)
	m.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	m.Handle("/debug/pprof/block", pprof.Handler("block"))
	m.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	m.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	m.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	m.Handle("/debug/vars", expvar.Handler())

	refresher := &tagRefresher{
		logger:             logger,
		k8sClient:          clientset,
		prometheusRegistry: prometheusRegistry,
		elbAPI:             elbAPI,
		waitBetweenRefresh: 1 * time.Minute,
		dryRun:             *dry,
	}

	go refresher.Start()

	port := GetenvOrDefault("PORT", DefaultPort)
	logger.Info("Server starting", "port", port)
	http.ListenAndServe(":"+port, Recoverer(logger)(m))
}

func GetenvOrDefault(env, def string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}

type tagRefresher struct {
	logger             log15.Logger
	k8sClient          *kubernetes.Clientset
	prometheusRegistry *prometheus.Registry
	elbAPI             *elb.ELB
	dryRun             bool
	waitBetweenRefresh time.Duration
}

func (r *tagRefresher) Start() {
	for {
		if err := r.refreshTags(); err != nil {
			r.logger.Error("Failed to refresh tags", "err", err)
		}

		time.Sleep(r.waitBetweenRefresh)
	}
}

func (r *tagRefresher) refreshTags() error {
	services, err := r.k8sClient.CoreV1().Services("").List(v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Error retrieving services from the Kubernetes cluster")
	}

	serviceTagsToApply := map[string]map[string]string{}

	for _, service := range services.Items {
		if service.Spec.Type != v1.ServiceTypeLoadBalancer {
			continue
		}
		awsTags := AWSTagsFromK8SAnnotations(service.Annotations)

		if len(awsTags) <= 0 {
			continue
		}

		// Get the ingress endpoints then tag the associated ELB accordingly
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			elbName, err := AWSELBNameFromHostname(ingress.Hostname)
			if err != nil {
				r.logger.Error("Error parsing the loadbalancer Hostname", "hostname", ingress.Hostname, "err", err)
			} else {
				serviceTagsToApply[elbName] = awsTags
			}

		}
	}

	// TODO: Ideally we should only change tags on elb which needs new tag, to do that we should query
	// the elb tags list before hand and filter on that
	for elbName, tags := range serviceTagsToApply {
		r.applyTagsToELB(elbName, tags)
	}

	return nil
}

func (r *tagRefresher) applyTagsToELB(elbName string, tags map[string]string) {
	awsTags := []*elb.Tag{}
	for k, v := range tags {
		awsTags = append(awsTags, &elb.Tag{
			Key:   &k,
			Value: &v,
		})
	}

	addTagInput := &elb.AddTagsInput{
		LoadBalancerNames: []*string{&elbName},
		Tags:              awsTags,
	}
	if r.dryRun {
		r.logger.Info("Tag To be added", "addTagInput", addTagInput)
	} else {
		out, err := r.elbAPI.AddTags(addTagInput)
		if err != nil {
			r.logger.Error("Error adding tags", "elb", elbName, "err", err, "out", out.String())
		}
	}
}

func Recoverer(logger log15.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rvr := recover()
				if rvr != nil {
					logger.Error(fmt.Sprintf("PANIC: %v", rvr), "panic", rvr, "stack", string(debug.Stack()))
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func AWSTagsFromK8SAnnotations(annotations map[string]string) map[string]string {
	tagsToApply := map[string]string{}

	splitKeys := map[string]string{}
	splitValues := map[string]string{}

	for k, v := range annotations {
		if strings.HasPrefix(k, ServiceAnnotationTagPrefix) && len(k) > len(ServiceAnnotationTagPrefix) {
			tagsToApply[k[len(ServiceAnnotationTagPrefix):]] = v
		}

		if strings.HasPrefix(k, ServiceAnnotationTagKeyPrefix) && len(k) > len(ServiceAnnotationTagKeyPrefix) {
			splitKeys[k[len(ServiceAnnotationTagKeyPrefix):]] = v
		}

		if strings.HasPrefix(k, ServiceAnnotationTagValuePrefix) && len(k) > len(ServiceAnnotationTagValuePrefix) {
			splitValues[k[len(ServiceAnnotationTagValuePrefix):]] = v
		}
	}

	for k, tagKey := range splitKeys {
		if tagVal, ok := splitValues[k]; ok {
			tagsToApply[tagKey] = tagVal
		}
	}

	return tagsToApply
}

func kubernetesConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		host, port := os.Getenv("KUBERNETES_HTTP_HOST"), os.Getenv("KUBERNETES_HTTP_PORT")

		if len(host) == 0 || len(port) == 0 {
			return nil, fmt.Errorf("Unable to load local proxy cluster configuration, KUBERNETES_HTTP_HOST & KUBERNETES_HTTP_PORT must be defined or if running in cluster KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT")
		}

		config = &rest.Config{Host: fmt.Sprintf("http://%s:%s", host, port)}
	}
	return config, nil
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<h1>AWS ELB Tagger</h1>
<ul>
	<li><a href='/healthz'>/healthz</a></li>
	<li><a href='/debug/pprof/'>/debug/pprof/</a></li>
	<li><a href='/metrics'>/metrics</a></li>
</ul>`))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ok"))
}

// AWSELBNameFromHostname returns the elb from the hostname
func AWSELBNameFromHostname(hostname string) (string, error) {
	hostnameSegments := strings.Split(hostname, "-")
	if len(hostnameSegments) < 2 {
		return "", fmt.Errorf("%s is not a valid ELB hostname", hostname)
	}

	if strings.HasPrefix(hostnameSegments[0], "internal") {
		return hostnameSegments[1], nil
	}

	return hostnameSegments[0], nil
}
