package service

import "context"

// AccountHealthLatencySample remains as a source-compatibility type while
// request-path health recording is retired. Probe observations never use it.
type AccountHealthLatencySample struct{}

func BuildAccountHealthLatencySample(_ int64, _ *int, _ int64) AccountHealthLatencySample {
	return AccountHealthLatencySample{}
}

func (s *GatewayService) RecordAccountHealthFailure(_ context.Context, _ int64, _ *UpstreamFailoverError) {
}

func (s *GatewayService) RecordAccountHealthForwardError(_ context.Context, _ int64, _ error) {
}

func (s *GatewayService) RecordAccountHealthSuccess(_ context.Context, _ int64, _ int64) {
}

func (s *GatewayService) RecordAccountHealthSuccessWithLatency(_ context.Context, _ int64, _ AccountHealthLatencySample) {
}

func (s *OpenAIGatewayService) RecordAccountHealthFailure(_ context.Context, _ int64, _ *UpstreamFailoverError) {
}

func (s *OpenAIGatewayService) RecordAccountHealthForwardError(_ context.Context, _ int64, _ error) {
}

func (s *OpenAIGatewayService) RecordAccountHealthSuccess(_ context.Context, _ int64, _ int64) {
}

func (s *OpenAIGatewayService) RecordAccountHealthSuccessWithLatency(_ context.Context, _ int64, _ AccountHealthLatencySample) {
}
