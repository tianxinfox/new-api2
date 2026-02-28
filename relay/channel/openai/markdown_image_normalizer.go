package openai

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

var markdownImageURLPattern = regexp.MustCompile(`!\[[^\]]*\]\((https?://[^)\s]+)\)`)

type imageURLPayload struct {
	Type   string                `json:"type"`
	Images []imageURLPayloadItem `json:"images"`
}

type imageURLPayloadItem struct {
	Url string `json:"url"`
}

// normalizeMarkdownImageChoices converts markdown image list responses into:
// {"type":"images","images":[{"url":"..."}]}
// Returns true when any choice content is rewritten.
func normalizeMarkdownImageChoices(resp *dto.OpenAITextResponse) bool {
	if resp == nil || len(resp.Choices) == 0 {
		return false
	}

	changed := false
	for i := range resp.Choices {
		raw := strings.TrimSpace(resp.Choices[i].Message.StringContent())
		if raw == "" {
			continue
		}

		urlMatches := markdownImageURLPattern.FindAllStringSubmatch(raw, -1)
		if len(urlMatches) == 0 {
			continue
		}

		// If there is too much meaningful text besides image markdown, keep original content.
		// This avoids rewriting normal mixed text+image answers.
		rest := markdownImageURLPattern.ReplaceAllString(raw, "")
		if countMeaningfulRunes(rest) > 32 {
			continue
		}

		seen := make(map[string]struct{}, len(urlMatches))
		images := make([]imageURLPayloadItem, 0, len(urlMatches))
		for _, m := range urlMatches {
			if len(m) < 2 {
				continue
			}
			url := strings.TrimSpace(m[1])
			if url == "" {
				continue
			}
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			images = append(images, imageURLPayloadItem{Url: url})
		}
		if len(images) == 0 {
			continue
		}

		payload := imageURLPayload{
			Type:   "images",
			Images: images,
		}
		buf, err := common.Marshal(payload)
		if err != nil {
			continue
		}

		resp.Choices[i].Message.SetStringContent(string(buf))
		changed = true
	}

	return changed
}

func countMeaningfulRunes(s string) int {
	count := 0
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			continue
		case unicode.IsLetter(r), unicode.IsNumber(r):
			count++
		}
	}
	return count
}
