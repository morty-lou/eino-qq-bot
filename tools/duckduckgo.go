package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DuckDuckGoSearchProvider 免费，无需 API Key
type DuckDuckGoSearchProvider struct{}

func NewDuckDuckGoSearchProvider() *DuckDuckGoSearchProvider {
	return &DuckDuckGoSearchProvider{}
}

func (p *DuckDuckGoSearchProvider) Search(_ context.Context, query string, count int) (string, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	return extractDDGResults(string(body), count)
}

func extractDDGResults(html string, count int) (string, error) {
	var results []string
	lines := strings.Split(html, "\n")
	var currentTitle, currentURL, currentSnippet string

	for _, line := range lines {
		if strings.Contains(line, `class="result__a"`) {
			hrefIdx := strings.Index(line, `href="`)
			if hrefIdx != -1 {
				hrefStart := hrefIdx + 6
				hrefEnd := strings.Index(line[hrefStart:], `"`)
				if hrefEnd != -1 {
					currentURL = line[hrefStart : hrefStart+hrefEnd]
					if strings.Contains(currentURL, "uddg=") {
						if u, err := url.QueryUnescape(strings.Split(currentURL, "uddg=")[1]); err == nil {
							currentURL = u
						}
					}
				}
			}
			titleStart := strings.Index(line, ">")
			titleEnd := strings.LastIndex(line, "<")
			if titleStart != -1 && titleEnd != -1 && titleStart < titleEnd {
				currentTitle = strings.TrimSpace(line[titleStart+1 : titleEnd])
				currentTitle = stripTags(currentTitle)
			}
		}
		if strings.Contains(line, `class="result__snippet"`) {
			snippetStart := strings.Index(line, ">")
			snippetEnd := strings.LastIndex(line, "<")
			if snippetStart != -1 && snippetEnd != -1 && snippetStart < snippetEnd {
				currentSnippet = strings.TrimSpace(line[snippetStart+1 : snippetEnd])
				currentSnippet = stripTags(currentSnippet)
			}
			if currentTitle != "" && currentURL != "" {
				results = append(results, fmt.Sprintf("%d. %s\n   %s", len(results)+1, currentTitle, currentURL))
				if currentSnippet != "" {
					results = append(results, fmt.Sprintf("   %s", currentSnippet))
				}
				currentTitle = ""
				currentURL = ""
				currentSnippet = ""
				if len(results)/2 >= count {
					break
				}
			}
		}
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for: %s", html), nil
	}

	output := []string{"Search results:"}
	for i := 0; i < len(results) && len(output) < count*3+1; i++ {
		output = append(output, results[i])
	}
	return strings.Join(output, "\n"), nil
}

func stripTags(content string) string {
	content = strings.ReplaceAll(content, "<b>", "")
	content = strings.ReplaceAll(content, "</b>", "")
	content = strings.ReplaceAll(content, "<strong>", "")
	content = strings.ReplaceAll(content, "</strong>", "")
	for {
		tagStart := strings.Index(content, "<")
		tagEnd := strings.Index(content, ">")
		if tagStart == -1 || tagEnd == -1 || tagStart > tagEnd {
			break
		}
		content = content[:tagStart] + content[tagEnd+1:]
	}
	return strings.TrimSpace(content)
}
