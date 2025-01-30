// Defines the messages and topics used for sending metrics

package main

import "regexp"

type MetricDefinition struct {
	Help *string `json:"help"`
	Type *string `json:"type"`
	Unit *string `json:"unit"`
}

type MetricPush struct {
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

var deviceIdFromTopicRegex = regexp.MustCompile(`^device/([^/]+)/`)
var deviceIdMetricNameRegex = regexp.MustCompile(`^device/([^/]+)/metrics/([^/]+)/`)
