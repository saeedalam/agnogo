// Package otel provides OpenTelemetry-compatible metrics export for agnogo agents.
//
// This package bridges agnogo's MetricsCollector and Trace hooks to the
// OpenTelemetry protocol, allowing you to ship agent telemetry to any
// OTLP-compatible backend (Datadog, Grafana, Jaeger, etc).
//
// Zero external dependencies — exports metrics in OTLP JSON format over HTTP
// using Go stdlib only. No OpenTelemetry SDK required.
//
// Quick start:
//
//	exporter := otel.NewExporter("http://localhost:4318/v1/metrics")
//	agent := agnogo.Agent("...", agnogo.WithTrace(exporter.Trace()))
//	defer exporter.Flush()
//
// With periodic push:
//
//	exporter := otel.NewExporter("http://localhost:4318/v1/metrics",
//	    otel.WithInterval(30 * time.Second),
//	    otel.WithServiceName("my-agent"),
//	)
//	defer exporter.Stop()
package otel
