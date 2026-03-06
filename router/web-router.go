package router

import (
	"bytes"
	"embed"
	"encoding/xml"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

var canonicalTagRegex = regexp.MustCompile(`(?i)<link[^>]*rel=["']canonical["'][^>]*>`)
var titleTagRegex = regexp.MustCompile(`(?is)<title[^>]*>.*?</title>`)
var descriptionTagRegex = regexp.MustCompile(`(?i)<meta[^>]*name=["']description["'][^>]*>`)
var robotsTagRegex = regexp.MustCompile(`(?i)<meta[^>]*name=["']robots["'][^>]*>`)
var keywordsTagRegex = regexp.MustCompile(`(?i)<meta[^>]*name=["']keywords["'][^>]*>`)
var ogTitleTagRegex = regexp.MustCompile(`(?i)<meta[^>]*property=["']og:title["'][^>]*>`)
var ogDescriptionTagRegex = regexp.MustCompile(`(?i)<meta[^>]*property=["']og:description["'][^>]*>`)
var ogURLTagRegex = regexp.MustCompile(`(?i)<meta[^>]*property=["']og:url["'][^>]*>`)
var ogImageTagRegex = regexp.MustCompile(`(?i)<meta[^>]*property=["']og:image["'][^>]*>`)
var ogSiteNameTagRegex = regexp.MustCompile(`(?i)<meta[^>]*property=["']og:site_name["'][^>]*>`)
var twitterTitleTagRegex = regexp.MustCompile(`(?i)<meta[^>]*name=["']twitter:title["'][^>]*>`)
var twitterDescriptionTagRegex = regexp.MustCompile(`(?i)<meta[^>]*name=["']twitter:description["'][^>]*>`)
var twitterImageTagRegex = regexp.MustCompile(`(?i)<meta[^>]*name=["']twitter:image["'][^>]*>`)
var webAppJSONLDTagRegex = regexp.MustCompile(`(?is)<script[^>]*id=["']seo-jsonld["'][^>]*>.*?</script>`)
var noScriptTagRegex = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)

func sanitizeScheme(raw string, useTLS bool) string {
	scheme := strings.ToLower(strings.TrimSpace(raw))
	if scheme == "http" || scheme == "https" {
		return scheme
	}
	if useTLS {
		return "https"
	}
	return "http"
}

func splitForwardedHost(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	return strings.TrimSpace(parts[0])
}

func isValidPort(raw string) bool {
	if raw == "" {
		return false
	}
	port, err := strconv.Atoi(raw)
	if err != nil {
		return false
	}
	return port >= 1 && port <= 65535
}

func isValidHostName(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	if host == "localhost" {
		return true
	}
	labels := strings.Split(host, ".")
	if len(labels) == 0 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, c := range label {
			isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
			if !isAlphaNum && c != '-' {
				return false
			}
		}
	}
	return true
}

func sanitizeHost(raw string) (string, bool) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", false
	}
	if strings.ContainsAny(host, "/\\<>\"'` \t\r\n") {
		return "", false
	}
	if parsedHost, port, err := net.SplitHostPort(host); err == nil {
		parsedHost = strings.Trim(parsedHost, "[]")
		if (net.ParseIP(parsedHost) != nil || isValidHostName(parsedHost)) && isValidPort(port) {
			return host, true
		}
		return "", false
	}
	trimmed := strings.Trim(host, "[]")
	if net.ParseIP(trimmed) != nil || isValidHostName(host) {
		return host, true
	}
	return "", false
}

func resolveSafeHost(c *gin.Context) string {
	if host, ok := sanitizeHost(splitForwardedHost(c.GetHeader("X-Forwarded-Host"))); ok {
		return host
	}
	if host, ok := sanitizeHost(c.Request.Host); ok {
		return host
	}
	return "localhost"
}

func xmlEscape(value string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return ""
	}
	return buf.String()
}

func buildCanonicalURL(c *gin.Context) string {
	scheme := sanitizeScheme(c.GetHeader("X-Forwarded-Proto"), c.Request.TLS != nil)
	host := resolveSafeHost(c)
	path := c.Request.URL.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	escapedPath := (&url.URL{Path: path}).EscapedPath()
	if escapedPath == "" {
		escapedPath = "/"
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, escapedPath)
}

func buildBaseURL(c *gin.Context) string {
	scheme := sanitizeScheme(c.GetHeader("X-Forwarded-Proto"), c.Request.TLS != nil)
	host := resolveSafeHost(c)
	return fmt.Sprintf("%s://%s", scheme, host)
}

func getCurrentLogo() string {
	common.OptionMapRWMutex.RLock()
	logo := strings.TrimSpace(common.OptionMap["Logo"])
	common.OptionMapRWMutex.RUnlock()
	if logo != "" {
		return logo
	}
	logo = strings.TrimSpace(common.Logo)
	if logo == "" {
		return "/logo.png"
	}
	return logo
}

func toAbsoluteAssetURL(baseURL string, asset string) string {
	normalized := strings.TrimSpace(asset)
	if normalized == "" {
		normalized = "/logo.png"
	}
	if strings.HasPrefix(normalized, "http://") || strings.HasPrefix(normalized, "https://") {
		return normalized
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	return baseURL + normalized
}

func getCurrentSystemName() string {
	common.OptionMapRWMutex.RLock()
	systemName := strings.TrimSpace(common.OptionMap["SystemName"])
	common.OptionMapRWMutex.RUnlock()
	if systemName != "" {
		return systemName
	}
	if strings.TrimSpace(common.SystemName) != "" {
		return common.SystemName
	}
	return "白泽 API"
}

func getDefaultSEODescription() string {
	return "统一的 AI API 网关，支持多供应商中继、额度控制、计费与运营管理。"
}

func getDefaultSEOKeywords() string {
	return "AI API,大模型接口,API网关,OpenAI,Claude,Gemini,API聚合,密钥管理,API代理,模型计费"
}

type pageSEOMeta struct {
	title       string
	description string
	robots      string
}

func shouldNoIndexPath(path string) bool {
	noIndexPrefixes := []string{"/console", "/api/", "/v1/", "/mj/", "/pg/", "/oauth"}
	noIndexPrefixExactPaths := map[string]struct{}{
		"/api": {},
		"/v1":  {},
		"/mj":  {},
		"/pg":  {},
	}
	noIndexExactPaths := map[string]struct{}{
		"/login":      {},
		"/register":   {},
		"/reset":      {},
		"/user/reset": {},
		"/setup":      {},
		"/chat2link":  {},
	}
	for _, prefix := range noIndexPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	if _, ok := noIndexPrefixExactPaths[path]; ok {
		return true
	}
	if _, ok := noIndexExactPaths[path]; ok {
		return true
	}
	return false
}

func buildServerSEOMeta(path string, systemName string) pageSEOMeta {
	robots := "index, follow"
	if shouldNoIndexPath(path) {
		robots = "noindex, nofollow"
	}
	pageMetaMap := map[string]pageSEOMeta{
		"/": {
			title:       fmt.Sprintf("%s - 大模型接口网关", systemName),
			description: "统一的 AI API 网关，聚合 OpenAI、Claude、Gemini、Azure、Bedrock 等渠道，支持密钥管理、计费与监控。",
			robots:      robots,
		},
		"/pricing": {
			title:       fmt.Sprintf("价格 - %s", systemName),
			description: "集中对比模型价格与 Token 成本，帮助你为 AI 业务选择更合适的渠道。",
			robots:      robots,
		},
		"/about": {
			title:       fmt.Sprintf("关于 - %s", systemName),
			description: "了解平台能力、项目背景与部署方式。",
			robots:      robots,
		},
		"/privacy-policy": {
			title:       fmt.Sprintf("隐私政策 - %s", systemName),
			description: "查看本平台如何收集、使用与保护个人数据。",
			robots:      robots,
		},
		"/user-agreement": {
			title:       fmt.Sprintf("用户协议 - %s", systemName),
			description: "使用平台前请阅读服务条款与使用条件。",
			robots:      robots,
		},
	}
	if meta, ok := pageMetaMap[path]; ok {
		return meta
	}
	return pageSEOMeta{
		title:       systemName,
		description: getDefaultSEODescription(),
		robots:      robots,
	}
}

func buildWebAppJSONLD(c *gin.Context, systemName string) string {
	baseURL := buildBaseURL(c)
	payload := map[string]any{
		"@context":            "https://schema.org",
		"@type":               "WebApplication",
		"name":                systemName,
		"url":                 baseURL,
		"description":         getDefaultSEODescription(),
		"applicationCategory": "DeveloperApplication",
		"operatingSystem":     "Any",
		"inLanguage":          "zh-CN",
		"offers": map[string]any{
			"@type":         "Offer",
			"availability":  "https://schema.org/OnlineOnly",
			"priceCurrency": "CNY",
		},
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func replaceOrInsertHeadTag(page string, tagRegex *regexp.Regexp, newTag string) string {
	if tagRegex.MatchString(page) {
		return tagRegex.ReplaceAllString(page, newTag)
	}
	if strings.Contains(page, "</head>") {
		return strings.Replace(page, "</head>", newTag+"</head>", 1)
	}
	return page
}

func replaceOrInsertNoScript(page string, noScriptTag string) string {
	if noScriptTagRegex.MatchString(page) {
		return noScriptTagRegex.ReplaceAllString(page, noScriptTag)
	}
	if strings.Contains(page, `<div id="root"></div>`) {
		return strings.Replace(page, `<div id="root"></div>`, noScriptTag+"\n    <div id=\"root\"></div>", 1)
	}
	if strings.Contains(page, "</body>") {
		return strings.Replace(page, "</body>", noScriptTag+"</body>", 1)
	}
	return page
}

func injectSEOIntoIndex(indexPage []byte, canonicalURL string, imageURL string, systemName string, title string, description string, robots string, jsonLD string) []byte {
	canonicalTag := fmt.Sprintf(`<link rel="canonical" href="%s" />`, html.EscapeString(canonicalURL))
	titleTag := fmt.Sprintf(`<title>%s</title>`, html.EscapeString(title))
	descriptionTag := fmt.Sprintf(`<meta name="description" content="%s" />`, html.EscapeString(description))
	robotsTag := fmt.Sprintf(`<meta name="robots" content="%s" />`, html.EscapeString(robots))
	keywordsTag := fmt.Sprintf(`<meta name="keywords" content="%s" />`, html.EscapeString(getDefaultSEOKeywords()))
	ogTitleTag := fmt.Sprintf(`<meta property="og:title" content="%s" />`, html.EscapeString(title))
	ogDescriptionTag := fmt.Sprintf(`<meta property="og:description" content="%s" />`, html.EscapeString(description))
	ogURLTag := fmt.Sprintf(`<meta property="og:url" content="%s" />`, html.EscapeString(canonicalURL))
	ogImageTag := fmt.Sprintf(`<meta property="og:image" content="%s" />`, html.EscapeString(imageURL))
	ogSiteNameTag := fmt.Sprintf(`<meta property="og:site_name" content="%s" />`, html.EscapeString(systemName))
	twitterTitleTag := fmt.Sprintf(`<meta name="twitter:title" content="%s" />`, html.EscapeString(title))
	twitterDescriptionTag := fmt.Sprintf(`<meta name="twitter:description" content="%s" />`, html.EscapeString(description))
	twitterImageTag := fmt.Sprintf(`<meta name="twitter:image" content="%s" />`, html.EscapeString(imageURL))
	noScriptTag := fmt.Sprintf(
		`<noscript><h1>%s</h1><p>%s</p><nav><a href="/pricing">价格</a> | <a href="/about">关于</a></nav></noscript>`,
		html.EscapeString(title),
		html.EscapeString(description),
	)
	page := string(indexPage)
	page = replaceOrInsertHeadTag(page, titleTagRegex, titleTag)
	page = replaceOrInsertHeadTag(page, descriptionTagRegex, descriptionTag)
	page = replaceOrInsertHeadTag(page, robotsTagRegex, robotsTag)
	page = replaceOrInsertHeadTag(page, keywordsTagRegex, keywordsTag)
	page = replaceOrInsertHeadTag(page, ogTitleTagRegex, ogTitleTag)
	page = replaceOrInsertHeadTag(page, ogDescriptionTagRegex, ogDescriptionTag)
	page = replaceOrInsertHeadTag(page, ogURLTagRegex, ogURLTag)
	page = replaceOrInsertHeadTag(page, ogImageTagRegex, ogImageTag)
	page = replaceOrInsertHeadTag(page, ogSiteNameTagRegex, ogSiteNameTag)
	page = replaceOrInsertHeadTag(page, twitterTitleTagRegex, twitterTitleTag)
	page = replaceOrInsertHeadTag(page, twitterDescriptionTagRegex, twitterDescriptionTag)
	page = replaceOrInsertHeadTag(page, twitterImageTagRegex, twitterImageTag)
	page = replaceOrInsertHeadTag(page, canonicalTagRegex, canonicalTag)
	if jsonLD != "" {
		jsonLDTag := fmt.Sprintf(`<script id="seo-jsonld" type="application/ld+json">%s</script>`, jsonLD)
		page = replaceOrInsertHeadTag(page, webAppJSONLDTagRegex, jsonLDTag)
	}
	page = replaceOrInsertNoScript(page, noScriptTag)
	return []byte(page)
}

func renderWebIndexWithSEO(c *gin.Context, indexPage []byte) {
	c.Header("Cache-Control", "no-cache")
	baseURL := buildBaseURL(c)
	canonicalURL := buildCanonicalURL(c)
	imageURL := toAbsoluteAssetURL(baseURL, getCurrentLogo())
	systemName := getCurrentSystemName()
	seoMeta := buildServerSEOMeta(c.Request.URL.Path, systemName)
	jsonLD := ""
	if c.Request.URL.Path == "/" {
		jsonLD = buildWebAppJSONLD(c, systemName)
	}
	indexWithSEO := injectSEOIntoIndex(indexPage, canonicalURL, imageURL, systemName, seoMeta.title, seoMeta.description, seoMeta.robots, jsonLD)
	c.Data(http.StatusOK, "text/html; charset=utf-8", indexWithSEO)
}

func SetWebRouter(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())

	router.GET("/robots.txt", func(c *gin.Context) {
		baseURL := buildBaseURL(c)
		robots := "User-agent: GPTBot\n" +
			"Allow: /\n" +
			"Disallow: /console\n" +
			"Disallow: /api/\n" +
			"Disallow: /v1/\n\n" +
			"User-agent: ClaudeBot\n" +
			"Allow: /\n" +
			"Disallow: /console\n" +
			"Disallow: /api/\n" +
			"Disallow: /v1/\n\n" +
			"User-agent: PerplexityBot\n" +
			"Allow: /\n" +
			"Disallow: /console\n\n" +
			"User-agent: Baiduspider\n" +
			"Allow: /\n" +
			"Disallow: /console\n" +
			"Disallow: /api/\n" +
			"Disallow: /v1/\n\n" +
			"User-agent: Baiduspider-render\n" +
			"Allow: /\n" +
			"Disallow: /console\n\n" +
			"User-agent: Bytespider\n" +
			"Allow: /\n" +
			"Disallow: /console\n" +
			"Disallow: /api/\n" +
			"Disallow: /v1/\n\n" +
			"User-agent: 360Spider\n" +
			"Allow: /\n" +
			"Disallow: /console\n" +
			"Disallow: /api/\n" +
			"Disallow: /v1/\n\n" +
			"User-agent: Sogou web spider\n" +
			"Allow: /\n" +
			"Disallow: /console\n\n" +
			"User-agent: YisouSpider\n" +
			"Allow: /\n" +
			"Disallow: /console\n\n" +
			"User-agent: *\n" +
			"Allow: /\n" +
			"Disallow: /console\n" +
			"Disallow: /api/\n" +
			"Disallow: /v1/\n" +
			"Disallow: /mj/\n" +
			"Disallow: /pg/\n\n" +
			"Sitemap: " + baseURL + "/sitemap.xml\n"
		c.Header("Cache-Control", "max-age=3600")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(robots))
	})

	router.GET("/sitemap.xml", func(c *gin.Context) {
		baseURL := buildBaseURL(c)
		lastModDate := time.Now().UTC().Format("2006-01-02")
		pages := []struct {
			path       string
			changeFreq string
			priority   string
		}{
			{path: "/", changeFreq: "daily", priority: "1.0"},
			{path: "/pricing", changeFreq: "daily", priority: "0.9"},
			{path: "/about", changeFreq: "weekly", priority: "0.7"},
			{path: "/user-agreement", changeFreq: "monthly", priority: "0.4"},
			{path: "/privacy-policy", changeFreq: "monthly", priority: "0.4"},
		}
		var builder strings.Builder
		builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		builder.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
		for _, page := range pages {
			builder.WriteString("<url>")
			builder.WriteString("<loc>")
			builder.WriteString(xmlEscape(baseURL + page.path))
			builder.WriteString("</loc>")
			builder.WriteString("<changefreq>")
			builder.WriteString(page.changeFreq)
			builder.WriteString("</changefreq>")
			builder.WriteString("<lastmod>")
			builder.WriteString(lastModDate)
			builder.WriteString("</lastmod>")
			builder.WriteString("<priority>")
			builder.WriteString(page.priority)
			builder.WriteString("</priority>")
			builder.WriteString("</url>")
		}
		builder.WriteString("</urlset>")
		c.Header("Cache-Control", "max-age=3600")
		c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(builder.String()))
	})
	router.GET("/llms.txt", func(c *gin.Context) {
		baseURL := buildBaseURL(c)
		systemName := getCurrentSystemName()
		llms := "# " + systemName + "\n" +
			"> 统一的 AI API 网关，聚合 40+ AI 供应商\n\n" +
			"## 功能\n" +
			"- 支持 OpenAI、Claude、Gemini、Azure、Bedrock 等渠道\n" +
			"- API Key 管理与二次分发\n" +
			"- 额度控制与计费\n" +
			"- 模型价格对比\n\n" +
			"## 链接\n" +
			"- 价格: " + baseURL + "/pricing\n" +
			"- 关于: " + baseURL + "/about\n"
		c.Header("Cache-Control", "max-age=3600")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(llms))
	})
	router.GET("/", func(c *gin.Context) {
		renderWebIndexWithSEO(c, indexPage)
	})
	router.Use(static.Serve("/", common.EmbedFolder(buildFS, "web/dist")))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		renderWebIndexWithSEO(c, indexPage)
	})
}
