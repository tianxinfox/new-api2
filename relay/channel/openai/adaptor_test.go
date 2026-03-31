package openai

import (
	"mime/multipart"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestBuildImageEditFormFieldValuesAppliesOverride(t *testing.T) {
	request := dto.ImageRequest{
		Model: "jimeng-4.5",
	}
	form := &multipart.Form{
		Value: map[string][]string{
			"prompt": {"加个帽子"},
			"size":   {"1024x1536"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []interface{}{
					map[string]interface{}{
						"path": "ratio",
						"mode": "size_to_ratio",
						"from": "size",
					},
					map[string]interface{}{
						"path": "resolution",
						"mode": "size_to_resolution",
						"from": "size",
					},
				},
			},
		},
	}

	values, err := buildImageEditFormFieldValues(request, form, info)
	if err != nil {
		t.Fatalf("buildImageEditFormFieldValues returned error: %v", err)
	}
	if got := values["ratio"]; len(got) != 1 || got[0] != "2:3" {
		t.Fatalf("unexpected ratio: %v", got)
	}
	if got := values["resolution"]; len(got) != 1 || got[0] != "2k" {
		t.Fatalf("unexpected resolution: %v", got)
	}
}

func TestBuildImageEditFormFieldValuesPreservesExplicitRatio(t *testing.T) {
	request := dto.ImageRequest{
		Model: "jimeng-4.5",
	}
	form := &multipart.Form{
		Value: map[string][]string{
			"prompt":     {"加个帽子"},
			"size":       {"1024x1024"},
			"ratio":      {"16:9"},
			"resolution": {"2k"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []interface{}{
					map[string]interface{}{
						"path": "ratio",
						"mode": "size_to_ratio",
						"from": "size",
					},
					map[string]interface{}{
						"path": "resolution",
						"mode": "size_to_resolution",
						"from": "size",
					},
				},
			},
		},
	}

	values, err := buildImageEditFormFieldValues(request, form, info)
	if err != nil {
		t.Fatalf("buildImageEditFormFieldValues returned error: %v", err)
	}
	if got := values["ratio"]; len(got) != 1 || got[0] != "16:9" {
		t.Fatalf("unexpected ratio: %v", got)
	}
	if got := values["resolution"]; len(got) != 1 || got[0] != "2k" {
		t.Fatalf("unexpected resolution: %v", got)
	}
}
