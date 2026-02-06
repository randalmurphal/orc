package code

import (
	"regexp"
	"strings"
)

// SecretDetector identifies and redacts secrets in source code.
type SecretDetector struct {
	patterns []secretPattern
}

type secretPattern struct {
	name    string
	regex   *regexp.Regexp
	typeStr string
}

var placeholderPatterns = []string{
	"your-", "example", "changeme", "${", "{{",
}

// NewSecretDetector creates a new secret detector.
func NewSecretDetector() *SecretDetector {
	return &SecretDetector{
		patterns: []secretPattern{
			{
				name:    "aws_access_key",
				regex:   regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
				typeStr: "aws_access_key",
			},
			{
				name:    "private_key",
				regex:   regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE KEY-----`),
				typeStr: "private_key",
			},
			{
				name:    "api_key",
				regex:   regexp.MustCompile(`(?i)(?:api[_-]?key|api[_-]?secret|secret[_-]?key)\s*[=:]\s*["']([a-zA-Z0-9_\-./]{20,})["']`),
				typeStr: "api_key",
			},
			{
				name:    "api_key_sk",
				regex:   regexp.MustCompile(`sk-[a-zA-Z0-9_\-]{20,}`),
				typeStr: "api_key",
			},
			{
				name:    "password",
				regex:   regexp.MustCompile(`(?i)(?:password|passwd|db_pass(?:word)?|database_password)\s*[=:]\s*["']([^"']+)["']`),
				typeStr: "password",
			},
			{
				name:    "aws_secret_key",
				regex:   regexp.MustCompile(`(?i)(?:aws[_-]?secret[_-]?access[_-]?key)\s*[=:]\s*["']([a-zA-Z0-9/+=]{30,})["']`),
				typeStr: "aws_secret_key",
			},
			{
				name:    "connection_string",
				regex:   regexp.MustCompile(`(?i)(?:postgresql|mysql|redis|mongodb|amqp)://[^:]*:[^@]+@[^\s"']+`),
				typeStr: "connection_string",
			},
		},
	}
}

// Detect scans content for secrets and returns findings.
func (d *SecretDetector) Detect(content string) []SecretFinding {
	lines := strings.Split(content, "\n")
	var findings []SecretFinding

	for lineIdx, line := range lines {
		for _, pat := range d.patterns {
			matches := pat.regex.FindAllString(line, -1)
			for _, match := range matches {
				if isPlaceholder(line) {
					continue
				}
				findings = append(findings, SecretFinding{
					Type:  pat.typeStr,
					Match: match,
					Line:  lineIdx + 1,
				})
			}
		}
	}

	return findings
}

// Redact replaces detected secret values with [REDACTED] in content.
func (d *SecretDetector) Redact(content string, findings []SecretFinding) string {
	result := content
	for _, f := range findings {
		result = strings.ReplaceAll(result, f.Match, "[REDACTED]")
	}
	return result
}

// HasSecrets returns true if there are any findings.
func (d *SecretDetector) HasSecrets(findings []SecretFinding) bool {
	return len(findings) > 0
}

func isPlaceholder(line string) bool {
	lower := strings.ToLower(line)
	// Extract the value portion of an assignment
	val := lower
	if idx := strings.Index(lower, "="); idx >= 0 {
		val = strings.TrimSpace(lower[idx+1:])
	} else if idx := strings.Index(lower, ":"); idx >= 0 {
		val = strings.TrimSpace(lower[idx+1:])
	}
	// Strip quotes
	val = strings.Trim(val, "\"' ")

	for _, ph := range placeholderPatterns {
		switch ph {
		case "your-":
			if strings.HasPrefix(val, "your-") || strings.HasPrefix(val, "your_") {
				return true
			}
		case "example":
			// Only match if the value starts with "example" (placeholder),
			// not if it merely contains "example" in a larger string
			if strings.HasPrefix(val, "example") {
				return true
			}
		case "changeme":
			if val == "changeme" {
				return true
			}
		case "${":
			if strings.Contains(val, "${") {
				return true
			}
		case "{{":
			if strings.Contains(val, "{{") {
				return true
			}
		}
	}
	return false
}
