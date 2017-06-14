package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/coveo/k8s-aws-elb-tagger/web/middleware"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/pressly/chi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
)

const (
	// DefaultPort is the port the service is listening by default
	DefaultPort = ":3000"

	// ServiceAnnotationTagPrefix is the prefix to put on k8s elb service annotations
	// to add tags to the AWS ELBs.
	//
	// ex:
	//  k8s annotation: aws-tag/owner="John Doe"
	//  AWS tag will be: owner="John Doe"
	ServiceAnnotationTagPrefix = "aws-tag/"

	ServiceAnnotationTagKeyPrefix   = "aws-tag-key/"
	ServiceAnnotationTagValuePrefix = "aws-tag-value/"

	DescribeTagInputMaxNames = 20
)

func main() {
	dry := os.Getenv("DRY_RUN")

	logger := log15.Root()
	logger.Info("Server Initializing")
	// Dependency Injection and initialization
	r := chi.NewRouter()
	prometheusRegistry := prometheus.NewRegistry()

	prometheusRegistry.MustRegister(prometheus.NewProcessCollector(os.Getpid(), ""))
	prometheusRegistry.MustRegister(prometheus.NewGoCollector())

	sess := session.Must(session.NewSession())
	elbAPI := elb.New(sess)
	//if elbAPI != nil {
	//	logger.Error("Unable to make connection to AWS", "client", 	elbAPI.ClientInfo )
	//	return
	//}

	// Setup middleware global and filters
	r.Use(
		middleware.Prometheus(prometheusRegistry),
		middleware.RequestID,
		middleware.RequestLogger(logger),
		middleware.Recoverer,
	)

	r.Get("/", homeHandler)
	r.Get("/healthz", healthHandler)
	r.Mount("/debug", middleware.Profiler())
	r.Mount("/metrics", promhttp.HandlerFor(prometheusRegistry, promhttp.HandlerOpts{}))

	config, err := kubernetesConfig()
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			services, err := clientset.CoreV1().Services("").List(v1.ListOptions{})
			if err != nil {
				logger.Error("Error retrieving services on cluster", "err", err)
			} else {

				serviceTagsToApply := map[string]map[string]string{}

				for _, service := range services.Items {
					if service.Spec.Type == v1.ServiceTypeLoadBalancer {

						tagsToApply := tagsToApplyFromAnnotations(service.Annotations)
						if len(tagsToApply) != 0 {
							// Get the ingress endpoints then tag the associated ELB accordingly
							for _, ingress := range service.Status.LoadBalancer.Ingress {
								loadbalancerHostname, err := loadBalancerNameFromHostname(ingress.Hostname)
								if err != nil {
									logger.Error("Error parsing the loadbalancer Hostname", "err", err)
								} else {
									serviceTagsToApply[loadbalancerHostname] = tagsToApply
								}
							}
						}
					}
				}

				logger.Info(fmt.Sprintf("%d Elbs to manage", len(serviceTagsToApply)))

				// TODO: Ideally we should only change tags on elb which needs new tag, to do that we should query
				// the elb tags list before hand
				for elbName, tags := range serviceTagsToApply {
					// FIXME: This we can do in parallel as long as we dont get throttled
					logger.Info("Applying tag to elb", "elb", elbName, "tags", tags)

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
					if strings.ToLower(dry) == "true" {
						logger.Info("Tag To be added", "addTagInput", addTagInput)
					} else {
						elbAPI.AddTags(addTagInput)
					}
				}
			}
			time.Sleep(1 * time.Minute)
		}
	}()

	port := DefaultPort
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}

	logger.Info("Server starting", "port", port)
	http.ListenAndServe(port, r)
	// FIXME: Implement graceful shutdown
}

func tagsToApplyFromAnnotations(annotations map[string]string) map[string]string {
	tagsToApply := map[string]string{}

	splitKeys := map[string]string{}
	splitValues := map[string]string{}

	for k, v := range annotations {
		if strings.HasPrefix(k, ServiceAnnotationTagPrefix) {
			tagsToApply[k[len(ServiceAnnotationTagPrefix):]] = v
		}

		if strings.HasPrefix(k, ServiceAnnotationTagKeyPrefix) {
			splitKeys[k[len(ServiceAnnotationTagKeyPrefix):]] = v
		}

		if strings.HasPrefix(k, ServiceAnnotationTagValuePrefix) {
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
			return nil, errors.Wrap(err, "Unable unable to load local proxy cluster configuration, KUBERNETES_HTTP_HOST & KUBERNETES_HTTP_PORT must be defined")
		}

		config = &rest.Config{
			Host: "http://" + net.JoinHostPort(host, port),
		}
	}
	return config, nil
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<h1>AWS ELB Tagger</h1>
<ul>
	<li><a href='/healthz'>/healthz</a></li>
	<li><a href='/debug'>/debug</a></li>
	<li><a href='/metrics'>/metrics</a></li>
</ul>`))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	// FIXME: Do a real health check
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ok"))
}

func PrettyJSONEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc
}

func loadBalancerNameFromHostname(hostname string) (string, error) {
	hostnameSegments := strings.Split(hostname, "-")
	if len(hostnameSegments) < 2 {
		return "", fmt.Errorf("%s is not a valid ELB hostname", hostname)
	}

	if strings.HasPrefix(hostnameSegments[0], "internal") {
		return hostnameSegments[1], nil
	}

	return hostnameSegments[0], nil
}
