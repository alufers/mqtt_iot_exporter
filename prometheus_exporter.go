// Contains stuff related to exporting metrics to Prometheus

package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	prometheus_dto "github.com/prometheus/client_model/go"
)

var allMetrics = map[string]*prometheus_dto.MetricFamily{}
var allMetricsLock = &sync.RWMutex{}

func parseMetricType(s string) (*prometheus_dto.MetricType, error) {
	if val, ok := prometheus_dto.MetricType_value[s]; ok {
		tmp := prometheus_dto.MetricType(val)
		return &tmp, nil
	}
	return nil, fmt.Errorf("unknown metric type: %s", s)
}

func runPrometheusServer() {
	mux := http.NewServeMux()
	deviceMetricGatherer := prometheus.GathererFunc(func() ([]*prometheus_dto.MetricFamily, error) {
		var metricsList []*prometheus_dto.MetricFamily
		for _, metric := range allMetrics {
			metricsList = append(metricsList, metric)
		}

		return metricsList, nil
	})

	prometheusHandler := promhttp.HandlerFor(
		prometheus.Gatherers([]prometheus.Gatherer{deviceMetricGatherer, systemRegistry}),
		promhttp.HandlerOpts{},
	)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// Wrap the prometheus handler to lock the metrics map
		allMetricsLock.RLock()
		defer allMetricsLock.RUnlock()
		prometheusHandler.ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/get-client-cert", getClientKeyEndpoint)
	http.ListenAndServe(config.MetricsAddr, mux)
}

func findSameMetricByNameAndLabels(name string, labels map[string]string) *prometheus_dto.Metric {
	if _, ok := allMetrics[name]; !ok {
		return nil
	}
	for _, metric := range allMetrics[name].Metric {
		if len(metric.Label) != len(labels) {
			continue
		}
		found := true
		for _, pair := range metric.Label {
			if val, ok := labels[*pair.Name]; !ok || val != *pair.Value {
				found = false
				break
			}
		}
		if found {
			return metric
		}
	}
	return nil
}

func labelsMapToLabelPairs(labels map[string]string) []*prometheus_dto.LabelPair {
	pairs := []*prometheus_dto.LabelPair{}
	for k, v := range labels {
		pairs = append(pairs, &prometheus_dto.LabelPair{
			Name:  &k,
			Value: &v,
		})
	}
	return pairs
}

// System metrics stuff
var systemRegistry = prometheus.NewRegistry()

var systemMetrics = struct {
	connectedMqttClients *prometheus.GaugeVec
	totalMqttConnections *prometheus.CounterVec
	uncleanDisconnects   *prometheus.CounterVec
}{
	connectedMqttClients: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mqtt_connected_clients",
		Help: "Number of currently connected MQTT clients to MQTT IoT exporter",
	}, []string{}),
	totalMqttConnections: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mqtt_total_connections",
		Help: "Total number of MQTT connections that were established to the MQTT IoT exporter",
	}, []string{}),
	uncleanDisconnects: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mqtt_unclean_disconnects",
		Help: "Total number of disconnects with a network error",
	}, []string{}),
}

func init() {
	systemRegistry.MustRegister(systemMetrics.connectedMqttClients)
	systemRegistry.MustRegister(systemMetrics.totalMqttConnections)
	systemRegistry.MustRegister(systemMetrics.uncleanDisconnects)

	// Initialize default values
	systemMetrics.connectedMqttClients.WithLabelValues().Set(0)
	systemMetrics.totalMqttConnections.WithLabelValues().Add(0)
	systemMetrics.uncleanDisconnects.WithLabelValues().Add(0)
}

// SystemMetricsHook is a hook which updates system metrics
// based on MQTT server events.
type SystemMetricsHook struct {
	mqtt.HookBase
}

func (h *SystemMetricsHook) ID() string {
	return "system-metrics"
}

func (h *SystemMetricsHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnect,
		mqtt.OnDisconnect,
	}, []byte{b})
}

func (h *SystemMetricsHook) OnConnect(*mqtt.Client, packets.Packet) error {
	systemMetrics.connectedMqttClients.WithLabelValues().Inc()
	systemMetrics.totalMqttConnections.WithLabelValues().Inc()
	return nil
}

func (h *SystemMetricsHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {

	systemMetrics.connectedMqttClients.WithLabelValues().Dec()

	if err != nil {
		systemMetrics.uncleanDisconnects.WithLabelValues().Inc()
	}
}
