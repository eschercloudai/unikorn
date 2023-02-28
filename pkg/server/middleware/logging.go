/*
Copyright 2022 EscherCloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package middleware

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/semconv/v1.18.0/httpconv"
	"go.opentelemetry.io/otel/trace"

	"github.com/eschercloudai/unikorn/pkg/constants"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// loggingResponseWriter is the ubiquitous reimplementation of a response
// writer that allows access to the HTTP status code in middleware.
type loggingResponseWriter struct {
	next http.ResponseWriter
	code int
}

// Check the correct interface is implmented.
var _ http.ResponseWriter = &loggingResponseWriter{}

func (w *loggingResponseWriter) Header() http.Header {
	return w.next.Header()
}

func (w *loggingResponseWriter) Write(body []byte) (int, error) {
	return w.next.Write(body)
}

func (w *loggingResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode
	w.next.WriteHeader(statusCode)
}

func (w *loggingResponseWriter) StatusCode() int {
	if w.code == 0 {
		return http.StatusOK
	}

	return w.code
}

// logValuesFromSpanContext gets a generic set of key/value pairs from a span for logging.
func logValuesFromSpanContext(s trace.SpanContext) []interface{} {
	return []interface{}{
		"span.id", s.SpanID().String(),
		"trace.id", s.TraceID().String(),
	}
}

// LoggingSpanProcessor is a OpenTelemetry span processor that logs to standard out
// in whatever format is defined by the logger.
type LoggingSpanProcessor struct{}

// Check the correct interface is implmented.
var _ sdktrace.SpanProcessor = &LoggingSpanProcessor{}

func (*LoggingSpanProcessor) OnStart(ctx context.Context, s sdktrace.ReadWriteSpan) {
	// Only log root spans.
	if s.Parent().IsValid() {
		return
	}

	attributes := logValuesFromSpanContext(s.SpanContext())

	for _, attribute := range s.Attributes() {
		attributes = append(attributes, string(attribute.Key), attribute.Value.Emit())
	}

	log.Log.Info("request started", attributes...)
}

func (*LoggingSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	// Only log root spans.
	if s.Parent().IsValid() {
		return
	}

	attributes := logValuesFromSpanContext(s.SpanContext())

	for _, attribute := range s.Attributes() {
		attributes = append(attributes, string(attribute.Key), attribute.Value.Emit())
	}

	log.Log.Info("request completed", attributes...)
}

func (*LoggingSpanProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func (*LoggingSpanProcessor) ForceFlush(ctx context.Context) error {
	return nil
}

// Logger attaches logging context to the request.
func Logger() func(next http.Handler) http.Handler {
	// TODO: this needs an implmenetation of https://www.w3.org/TR/trace-context/.
	// Like everything here, OpenTelemetry is very good at doing nothing by default.
	propagator := otel.GetTextMapPropagator()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract the tracing information from the HTTP headers.  See above
			// for what this entails.
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			var attributes []attribute.KeyValue

			// Add in service information.
			attributes = append(attributes, semconv.ServiceName(constants.Application))
			attributes = append(attributes, semconv.ServiceVersion(constants.Version))

			// Extract information from the HTTP request for logging purposes.
			attributes = append(attributes, httpconv.ServerRequest("", r)...)
			attributes = append(attributes, httpconv.RequestHeader(r.Header)...)

			tracer := otel.GetTracerProvider().Tracer(constants.Application)

			// Begin the span processing.
			ctx, span := tracer.Start(ctx, r.URL.Path,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attributes...),
			)
			defer span.End()

			// Setup logging.
			ctx = log.IntoContext(ctx, log.Log.WithValues(logValuesFromSpanContext(span.SpanContext())...))

			// Create a new request with any contextual information the tracer has added.
			request := r.WithContext(ctx)

			writer := &loggingResponseWriter{
				next: w,
			}

			next.ServeHTTP(writer, request)

			// Extract HTTP response information for logging purposes.
			span.SetStatus(httpconv.ServerStatus(writer.StatusCode()))
			span.SetAttributes(httpconv.ResponseHeader(writer.Header())...)
		})
	}
}
