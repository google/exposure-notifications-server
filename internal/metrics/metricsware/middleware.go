// Package metricsware provides a middleware for recording metrics of different kinds
package metricsware

import "github.com/google/exposure-notifications-server/internal/metrics"

type Middleware struct {
	exporter *metrics.Exporter
}

func NewMiddleWare(exporter *metrics.Exporter) Middleware {
	return Middleware{
		exporter: exporter,
	}
}
