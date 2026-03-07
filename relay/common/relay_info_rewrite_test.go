package common

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestRewriteRequestURLPathForUpstream_ConfigExact(t *testing.T) {
	settings := dto.ChannelOtherSettings{
		EndpointPathRewrite: map[string]string{
			"/v1/images/edits": "/v1/images/compositions",
		},
	}

	got := rewriteRequestURLPathForUpstream("/v1/images/edits", settings)
	if got != "/v1/images/compositions" {
		t.Fatalf("unexpected rewritten path: %s", got)
	}
}

func TestRewriteRequestURLPathForUpstream_ConfigWildcardWithQuery(t *testing.T) {
	settings := dto.ChannelOtherSettings{
		EndpointPathRewrite: map[string]string{
			"/v1/images/edits*": "/v1/images/compositions*",
		},
	}

	got := rewriteRequestURLPathForUpstream("/v1/images/edits/extra?x=1", settings)
	if got != "/v1/images/compositions/extra?x=1" {
		t.Fatalf("unexpected rewritten path with query: %s", got)
	}
}

func TestRewriteRequestURLPathForUpstream_NoConfiguredRewrite(t *testing.T) {
	settings := dto.ChannelOtherSettings{}

	got := rewriteRequestURLPathForUpstream("/v1/images/edits?size=1024", settings)
	if got != "/v1/images/edits?size=1024" {
		t.Fatalf("unexpected rewritten path without config: %s", got)
	}
}
