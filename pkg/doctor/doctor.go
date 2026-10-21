package doctor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// CheckResult содержит результат сетевой проверки
type CheckResult struct {
	Name    string
	URL     string
	Success bool
	Latency time.Duration
	Error   string
}

// RunNetworkDiagnostics проверяет доступность ключевых серверов Google/Antigravity
func RunNetworkDiagnostics() []CheckResult {
	endpoints := []struct {
		name string
		url  string
	}{
		{"Google OAuth2", "https://oauth2.googleapis.com"},
		{"Google APIs", "https://www.googleapis.com"},
		{"Gemini API", "https://generativelanguage.googleapis.com"},
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 4 * time.Second,
			}).DialContext,
		},
	}

	var results []CheckResult
	for _, ep := range endpoints {
		start := time.Now()
		req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, ep.url, nil)
		if err != nil {
			results = append(results, CheckResult{
				Name:    ep.name,
				URL:     ep.url,
				Success: false,
				Error:   err.Error(),
			})
			continue
		}

		resp, err := client.Do(req)
		latency := time.Since(start)

		if err != nil {
			results = append(results, CheckResult{
				Name:    ep.name,
				URL:     ep.url,
				Success: false,
				Latency: latency,
				Error:   fmt.Sprintf("connection failed (%v)", err),
			})
			continue
		}
		_ = resp.Body.Close()

		results = append(results, CheckResult{
			Name:    ep.name,
			URL:     ep.url,
			Success: true,
			Latency: latency,
		})
	}

	return results
}
