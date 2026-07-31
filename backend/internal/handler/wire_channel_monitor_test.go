package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestSetChannelMonitorSignerUsesMonitorServiceSigner(t *testing.T) {
	monitor := service.NewChannelMonitorService(nil, nil)
	gateway := &GatewayHandler{}
	openAI := &OpenAIGatewayHandler{}

	setChannelMonitorSigner(gateway, monitor)
	setChannelMonitorSigner(openAI, monitor)

	if gateway.channelMonitorSigner != monitor.Signer() {
		t.Fatal("expected gateway handler to use channel monitor signer")
	}
	if openAI.channelMonitorSigner != monitor.Signer() {
		t.Fatal("expected OpenAI gateway handler to use channel monitor signer")
	}
}

func TestSetChannelMonitorSignerWithNilMonitorIsNoop(t *testing.T) {
	gateway := &GatewayHandler{}
	setChannelMonitorSigner(gateway, nil)

	if gateway.channelMonitorSigner != nil {
		t.Fatal("expected nil monitor to leave signer unset")
	}
}
