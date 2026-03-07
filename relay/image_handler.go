package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ImageHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	imageReq, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.ImageRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(imageReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to ImageRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	var requestBody io.Reader

	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		contentType := strings.ToLower(c.Request.Header.Get("Content-Type"))
		isJSONBody := strings.Contains(contentType, "application/json") || strings.Contains(contentType, "+json")

		// In pass-through mode, still allow JSON param override for endpoint-level field remapping.
		if isJSONBody {
			rawBody, err := storage.Bytes()
			if err != nil {
				return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}

			// Optional channel-level conversion:
			// /v1/images/* JSON -> /v1/chat/completions request schema.
			conversionEnabled := info.ChannelOtherSettings.ImageToChatEnabled || info.ChannelOtherSettings.ImageEditsToChatEnabled
			if conversionEnabled &&
				isImageToChatRelayMode(info.RelayMode) &&
				isChatCompletionsRequestPath(info.RequestURLPath) {
				rawBody, err = transformImageToChatCompletionsBody(rawBody)
				if err != nil {
					return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
				}
			}

			if len(info.ParamOverride) > 0 {
				rawBody, err = relaycommon.ApplyParamOverride(rawBody, info.ParamOverride, relaycommon.BuildParamOverrideContext(info))
				if err != nil {
					return types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
				}
			}
			requestBody = bytes.NewBuffer(rawBody)
		} else {
			requestBody = common.ReaderOnly(storage)
		}
	} else {
		convertedRequest, err := adaptor.ConvertImageRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed)
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		switch convertedRequest.(type) {
		case *bytes.Buffer:
			requestBody = convertedRequest.(io.Reader)
		default:
			jsonData, err := common.Marshal(convertedRequest)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}

			// apply param override
			if len(info.ParamOverride) > 0 {
				jsonData, err = relaycommon.ApplyParamOverride(jsonData, info.ParamOverride, relaycommon.BuildParamOverrideContext(info))
				if err != nil {
					return types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
				}
			}

			if common.DebugEnabled {
				logger.LogDebug(c, fmt.Sprintf("image request body: %s", string(jsonData)))
			}
			requestBody = bytes.NewBuffer(jsonData)
		}
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			if httpResp.StatusCode == http.StatusCreated && info.ApiType == constant.APITypeReplicate {
				// replicate channel returns 201 Created when using Prefer: wait, treat it as success.
				httpResp.StatusCode = http.StatusOK
			} else {
				newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
				// reset status code 重置状态码
				service.ResetStatusCode(newAPIError, statusCodeMappingStr)
				return newAPIError
			}
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	if usage.(*dto.Usage).TotalTokens == 0 {
		usage.(*dto.Usage).TotalTokens = int(request.N)
	}
	if usage.(*dto.Usage).PromptTokens == 0 {
		usage.(*dto.Usage).PromptTokens = int(request.N)
	}

	quality := "standard"
	if request.Quality == "hd" {
		quality = "hd"
	}

	var logContent []string

	if len(request.Size) > 0 {
		logContent = append(logContent, fmt.Sprintf("大小 %s", request.Size))
	}
	if len(quality) > 0 {
		logContent = append(logContent, fmt.Sprintf("品质 %s", quality))
	}
	if request.N > 0 {
		logContent = append(logContent, fmt.Sprintf("生成数量 %d", request.N))
	}

	postConsumeQuota(c, info, usage.(*dto.Usage), logContent...)
	return nil
}

func isChatCompletionsRequestPath(requestURLPath string) bool {
	path, _, _ := strings.Cut(requestURLPath, "?")
	return path == "/v1/chat/completions" || strings.HasPrefix(path, "/v1/chat/completions/")
}

func isImageToChatRelayMode(relayMode int) bool {
	return relayMode == relayconstant.RelayModeImagesEdits || relayMode == relayconstant.RelayModeImagesGenerations
}

func transformImageToChatCompletionsBody(rawBody []byte) ([]byte, error) {
	var body map[string]interface{}
	if err := common.Unmarshal(rawBody, &body); err != nil {
		return nil, fmt.Errorf("invalid json body: %w", err)
	}

	model := strings.TrimSpace(interfaceToString(body["model"]))
	if model == "" {
		return nil, fmt.Errorf("model is required for image_to_chat conversion")
	}

	prompt := strings.TrimSpace(interfaceToString(body["prompt"]))
	ratio := strings.TrimSpace(interfaceToString(body["ratio"]))
	aspectRatio := strings.TrimSpace(interfaceToString(body["aspect_ratio"]))
	if aspectRatio == "" {
		aspectRatio = ratio
	}
	resolution := strings.TrimSpace(interfaceToString(body["resolution"]))

	textParts := make([]string, 0, 3)
	if prompt != "" {
		textParts = append(textParts, prompt)
	}
	if aspectRatio != "" {
		textParts = append(textParts, aspectRatio)
	}
	if resolution != "" {
		textParts = append(textParts, resolution)
	}
	textContent := strings.Join(textParts, ",")

	imageURLs := collectImageURLs(body)

	content := make([]map[string]interface{}, 0, len(imageURLs)+1)
	if textContent != "" {
		content = append(content, map[string]interface{}{
			"type": "text",
			"text": textContent,
		})
	}
	for _, url := range imageURLs {
		content = append(content, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": url,
			},
		})
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("no content generated for chat/completions conversion")
	}

	target := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": content,
			},
		},
	}

	// Preserve stream flag if caller sets it.
	if streamVal, ok := body["stream"]; ok {
		target["stream"] = streamVal
	}

	return common.Marshal(target)
}

func collectImageURLs(body map[string]interface{}) []string {
	urlSet := make(map[string]struct{})
	result := make([]string, 0)

	appendURL := func(value string) {
		url := strings.TrimSpace(value)
		if url == "" {
			return
		}
		if !isSupportedImageURL(url) {
			return
		}
		if _, exists := urlSet[url]; exists {
			return
		}
		urlSet[url] = struct{}{}
		result = append(result, url)
	}

	appendFromAny := func(value interface{}) {
		switch v := value.(type) {
		case string:
			appendURL(v)
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					appendURL(s)
				}
			}
		}
	}

	if imageVal, ok := body["image"]; ok {
		appendFromAny(imageVal)
	}
	if imagesVal, ok := body["images"]; ok {
		appendFromAny(imagesVal)
	}

	// Also support common indexed keys from clients like image_1, image_2 ...
	indexedKeys := make([]string, 0)
	for k := range body {
		if indexedImageKeyRegex.MatchString(k) {
			indexedKeys = append(indexedKeys, k)
		}
	}
	sort.Strings(indexedKeys)
	for _, k := range indexedKeys {
		appendFromAny(body[k])
	}

	return result
}

var indexedImageKeyRegex = regexp.MustCompile(`^image_\d+$`)

func isSupportedImageURL(url string) bool {
	lower := strings.ToLower(strings.TrimSpace(url))
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "data:image/")
}

func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
