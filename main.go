package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
	prometheus_dto "github.com/prometheus/client_model/go"
)

var server *mqtt.Server

func metricsDefineFn(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) error {
	// log.Print("inline client received message from subscription", "client", cl.ID, "subscriptionId", sub.Identifier, "topic", pk.TopicName, "payload", string(pk.Payload))
	log.Printf("got message from client %s on topic %s with payload %s", cl.ID, pk.TopicName, string(pk.Payload))

	parsed := MetricDefinition{}
	err := json.Unmarshal(pk.Payload, &parsed)
	if err != nil {

		return fmt.Errorf("failed to parse metric definition: %s", err)
	}
	parsedTopic := deviceIdMetricNameRegex.FindStringSubmatch(pk.TopicName)

	var metricName string
	if parsedTopic != nil {
		metricName = parsedTopic[2]
	}
	if metricName == "" {
		return fmt.Errorf("metric definition must have a name")
	}

	allMetricsLock.Lock()
	defer allMetricsLock.Unlock()
	if _, ok := allMetrics[metricName]; !ok {
		allMetrics[metricName] = &prometheus_dto.MetricFamily{
			Metric: []*prometheus_dto.Metric{},
		}
	}
	allMetrics[metricName].Name = &metricName

	allMetrics[metricName].Help = parsed.Help
	if parsed.Type != nil {
		allMetrics[metricName].Type, err = parseMetricType(strings.ToUpper(*parsed.Type))
		if err != nil {
			return fmt.Errorf("unknown metric type: %s", err)
		}
	}
	allMetrics[metricName].Unit = parsed.Unit

	return nil
}

func metricsPushFn(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) error {

	var parsed MetricPush
	err := json.Unmarshal(pk.Payload, &parsed)
	if err != nil {
		return fmt.Errorf("failed to parse metric push: %s", err)
	}

	labels := parsed.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	var parsedTopic = deviceIdMetricNameRegex.FindStringSubmatch(pk.TopicName)
	if parsedTopic == nil {
		return fmt.Errorf("invalid topic")
	}
	deviceId := parsedTopic[1]
	metricName := parsedTopic[2]

	if _, ok := allMetrics[metricName]; !ok {
		return fmt.Errorf("unknown metric, please use /define first: %s", metricName)
	}
	labels["device_id"] = deviceId

	allMetricsLock.Lock()
	defer allMetricsLock.Unlock()
	// try to find existing metric, otherwise create a new one
	metric := findSameMetricByNameAndLabels(metricName, labels)
	if metric == nil {
		metric = &prometheus_dto.Metric{}
		metric.Label = labelsMapToLabelPairs(labels)
		allMetrics[metricName].Metric = append(allMetrics[metricName].Metric, metric)
	}

	metricType := prometheus_dto.MetricType_UNTYPED
	if allMetrics[metricName].Type != nil {
		metricType = *allMetrics[metricName].Type
	}

	// assign value
	switch metricType {
	case prometheus_dto.MetricType_COUNTER:
		metric.Counter = &prometheus_dto.Counter{Value: &parsed.Value}
	case prometheus_dto.MetricType_GAUGE:
		metric.Gauge = &prometheus_dto.Gauge{Value: &parsed.Value}
	default:
		metric.Untyped = &prometheus_dto.Untyped{Value: &parsed.Value}
	}
	return nil

}

func mqttErrorHandlerWrapper(fn func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) error) func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
	return func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
		defer func() {
			if r := recover(); r != nil {
				// Print the stack trace
				debug.PrintStack()
				log.Printf("panic while handling message on topic %s: %s", pk.TopicName, r)
			}
		}()
		err := fn(cl, sub, pk)
		if err != nil {
			log.Printf("handling message on topic %s failed: %s", pk.TopicName, err)

			deviceId := deviceIdFromTopicRegex.FindStringSubmatch(pk.TopicName)[1]
			topic := "device/" + deviceId + "/server_error"
			payload, _ := json.Marshal(map[string]string{
				"topic": pk.TopicName,
				"error": err.Error(),
			})
			server.Publish(topic, payload, false, 0)
		}
	}
}

func main() {

	loadConfig()
	autogenerateClientCaIfNeeded()

	// Create signals channel to run server until interrupted
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
	}()

	go runPrometheusServer()

	// Create the new MQTT Server.
	server = mqtt.New(
		&mqtt.Options{
			InlineClient: true, // you must enable inline client to use direct publishing and subscribing.
		},
	)

	// Allow all connections.
	_ = server.AddHook(&IoTExporterAuthHook{}, nil)
	_ = server.AddHook(&SystemMetricsHook{}, nil)

	var tlsConfig *tls.Config

	if config.ServerCert != "" && config.ServerKey != "" {
		log.Printf("Server certificate and key provided, enabling TLS")
		cert, err := tls.LoadX509KeyPair(config.ServerCert, config.ServerKey)
		if err != nil {
			log.Fatalf("failed to load server certificate and key from %s and %s: %s", config.ServerCert, config.ServerKey, err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		if config.ClientCACert != "" {
			log.Printf("Client CA certificate provided, enabling client certificate verification")
			caCert, err := os.ReadFile(config.ClientCACert)
			if err != nil {
				log.Fatalf("failed to read client CA certificate from %s: %s", config.ClientCACert, err)
			}
			tlsConfig.ClientCAs = x509.NewCertPool()
			if !tlsConfig.ClientCAs.AppendCertsFromPEM(caCert) {
				log.Fatalf("failed to parse client CA certificate from %s", config.ClientCACert)
			}
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}

	// Create a TCP listener on a standard port.
	tcp := listeners.NewTCP(listeners.Config{ID: "t1", Address: config.MqttAddr, TLSConfig: tlsConfig})

	err := server.AddListener(tcp)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		log.Printf("starting mqtt server on %s", config.MqttAddr)
		err := server.Serve()
		if err != nil {
			log.Fatal(err)
		}
	}()

	server.Subscribe("device/+/metrics/+/define", 1, mqttErrorHandlerWrapper(metricsDefineFn))
	server.Subscribe("device/+/metrics/+/push", 1, mqttErrorHandlerWrapper(metricsPushFn))

	// Run server until interrupted
	<-done

	// Cleanup
}
