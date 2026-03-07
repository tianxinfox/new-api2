package relay

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestTransformImageToChatCompletionsBody(t *testing.T) {
	raw := []byte(`{
		"model":"jimeng-4.5",
		"prompt":"图一的女生拿着图二的玉佩",
		"image":["http://a/1.jpg","http://a/2.jpg"],
		"aspect_ratio":"16:9",
		"resolution":"2k"
	}`)

	converted, err := transformImageToChatCompletionsBody(raw)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}

	var body map[string]interface{}
	if err = common.Unmarshal(converted, &body); err != nil {
		t.Fatalf("unmarshal converted failed: %v", err)
	}

	if body["model"] != "jimeng-4.5" {
		t.Fatalf("unexpected model: %v", body["model"])
	}

	messages, ok := body["messages"].([]interface{})
	if !ok || len(messages) != 1 {
		t.Fatalf("messages invalid: %#v", body["messages"])
	}

	msg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("message[0] invalid type: %#v", messages[0])
	}
	if msg["role"] != "user" {
		t.Fatalf("unexpected role: %v", msg["role"])
	}

	content, ok := msg["content"].([]interface{})
	if !ok || len(content) != 3 {
		t.Fatalf("content invalid: %#v", msg["content"])
	}

	textItem, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("content[0] invalid type")
	}
	text, _ := textItem["text"].(string)
	if !strings.Contains(text, "图一的女生拿着图二的玉佩") || !strings.Contains(text, "16:9") || !strings.Contains(text, "2k") {
		t.Fatalf("unexpected text: %q", text)
	}
}

func TestTransformImageToChatCompletionsBody_UseImagesField(t *testing.T) {
	raw := []byte(`{
		"model":"jimeng-4.5",
		"prompt":"x",
		"images":["http://a/1.jpg","http://a/2.jpg","http://a/2.jpg"]
	}`)

	converted, err := transformImageToChatCompletionsBody(raw)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}

	var body map[string]interface{}
	if err = common.Unmarshal(converted, &body); err != nil {
		t.Fatalf("unmarshal converted failed: %v", err)
	}

	messages := body["messages"].([]interface{})
	msg := messages[0].(map[string]interface{})
	content := msg["content"].([]interface{})
	// text + 2 unique image_url items
	if len(content) != 3 {
		t.Fatalf("unexpected content length: %d", len(content))
	}
}

func TestCollectImageURLs_FilterNonURLAndIndexedKeys(t *testing.T) {
	body := map[string]interface{}{
		"image":        []interface{}{"http://a/1.jpg", "not-a-url"},
		"image_1":      "https://a/2.jpg",
		"image_format": "png",
		"image_foo":    "http://a/3.jpg",
	}

	got := collectImageURLs(body)
	if len(got) != 2 {
		t.Fatalf("unexpected image url count: %d, urls=%v", len(got), got)
	}
	if got[0] != "http://a/1.jpg" || got[1] != "https://a/2.jpg" {
		t.Fatalf("unexpected urls: %v", got)
	}
}

func TestTransformImageToChatCompletionsBody_GenerationsRatio(t *testing.T) {
	raw := []byte(`{
		"model":"jimeng-4.5",
		"prompt":"一只可爱的小猫咪",
		"ratio":"16:9",
		"resolution":"4k"
	}`)

	converted, err := transformImageToChatCompletionsBody(raw)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}

	var body map[string]interface{}
	if err = common.Unmarshal(converted, &body); err != nil {
		t.Fatalf("unmarshal converted failed: %v", err)
	}

	messages := body["messages"].([]interface{})
	msg := messages[0].(map[string]interface{})
	content := msg["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("unexpected content length: %d", len(content))
	}
	text := content[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(text, "一只可爱的小猫咪") || !strings.Contains(text, "16:9") || !strings.Contains(text, "4k") {
		t.Fatalf("unexpected text: %q", text)
	}
}
