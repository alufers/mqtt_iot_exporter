package main

import (
	"log"

	"github.com/kelseyhightower/envconfig"
)

type MqttIotExporterConfig struct {
	MetricsAddr string `default:"127.0.0.1:9100" envconfig:"METRICS_ADDR"`

	MqttAddr string `default:":1883" envconfig:"MQTT_ADDR"`

	ServerCert string `envconfig:"SERVER_CERT_FILE"`
	ServerKey  string `envconfig:"SERVER_KEY_FILE"`

	ClientCACert              string `envconfig:"CLIENT_CA_CERT"`
	ClientCAKey               string `envconfig:"CLIENT_CA_KEY"`
	AutogenerateClientCA      bool   `envconfig:"AUTOGENERATE_CLIENT_CA"`       // Automatically create a client CA if it does not exist
	EnableClientKeyGeneration bool   `envconfig:"ENABLE_CLIENT_KEY_GENERATION"` // if set the metrics HTTP server will expose /api/get-client-cert
}

var config MqttIotExporterConfig

func loadConfig() {
	err := envconfig.Process("mqtt_iot_exporter", &config)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%#v", config)
}
