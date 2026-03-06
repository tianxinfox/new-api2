package openai

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/dto"
)

var markdownImageURLPattern = regexp.MustCompile(`!\[[^\]]*\]\((https?://[^)\s]+)\)`)

// normalizeMarkdownImageChoices extracts markdown image URLs for later injection
// into the top-level response data field while preserving the original content.
func normalizeMarkdownImageChoices(resp *dto.OpenAITextResponse) ([]dto.ImageURLDataItem, bool) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, false
	}

	data := make([]dto.ImageURLDataItem, 0)
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

		// Keep de-duplication scoped to a single choice so multi-choice
		// responses preserve the previous counting behavior.
		seen := make(map[string]struct{}, len(urlMatches))
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
			data = append(data, dto.ImageURLDataItem{Url: url})
		}
	}

	if len(data) == 0 {
		return nil, false
	}
	resp.Usage.GeneratedImages = len(data)
	return data, true
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
