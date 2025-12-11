package myotel

import (
	"strings"

	"go.opentelemetry.io/otel/sdk/trace"
)

type headerSampler struct {
	key, value string
}

// NewHeaderSampler 返回一个 trace.Sampler
func NewHeaderSampler(headerKey, headerValue string) trace.Sampler {
	return &headerSampler{key: headerKey, value: headerValue}
}

func (s *headerSampler) ShouldSample(p trace.SamplingParameters) trace.SamplingResult {
	// 从 parent context 里取 attributes（Gin 中间件提前写好的）
	v := p.ParentContext.Value("http.header." + s.key)
	if str, ok := v.(string); ok && strings.TrimSpace(str) == s.value {
		return trace.SamplingResult{Decision: trace.RecordAndSample}
	}
	return trace.SamplingResult{Decision: trace.Drop}
}

func (s *headerSampler) Description() string {
	return "HeaderSampler"
}
