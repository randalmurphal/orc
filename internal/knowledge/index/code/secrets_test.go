package code

import (
	"strings"
	"testing"
)

// --- SC-6: Secret detector ---

// SC-6: Detects API keys.
func TestSecrets_APIKeys(t *testing.T) {
	content := `config = {
    "api_key": "sk-proj-abc123def456ghi789jkl012mno345pqr678stu901vwx",
    "name": "my-app"
}
`
	d := NewSecretDetector()
	findings := d.Detect(content)

	if len(findings) == 0 {
		t.Fatal("should detect API key")
	}

	found := false
	for _, f := range findings {
		if f.Type == "api_key" || strings.Contains(strings.ToLower(f.Type), "api") {
			found = true
			break
		}
	}
	if !found {
		t.Error("should detect finding of type api_key")
	}
}

// SC-6: Detects AWS access keys.
func TestSecrets_AWSAccessKeys(t *testing.T) {
	content := `AWS_ACCESS_KEY_ID = "AKIAIOSFODNN7EXAMPLE"
AWS_SECRET_ACCESS_KEY = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
`
	d := NewSecretDetector()
	findings := d.Detect(content)

	if len(findings) < 1 {
		t.Fatal("should detect AWS access key")
	}

	// At minimum, the AKIA pattern should be detected
	hasAWS := false
	for _, f := range findings {
		if strings.Contains(strings.ToLower(f.Type), "aws") || strings.Contains(f.Match, "AKIA") {
			hasAWS = true
			break
		}
	}
	if !hasAWS {
		t.Error("should detect AWS access key pattern (AKIA...)")
	}
}

// SC-6: Detects private keys.
func TestSecrets_PrivateKeys(t *testing.T) {
	content := `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA2f3h3v4m5n6o7p8q9r0s1t2u3v4w5x6y7z8A9B0C1D2E3F
-----END RSA PRIVATE KEY-----
`
	d := NewSecretDetector()
	findings := d.Detect(content)

	if len(findings) == 0 {
		t.Fatal("should detect private key")
	}

	hasPrivateKey := false
	for _, f := range findings {
		if strings.Contains(strings.ToLower(f.Type), "private_key") || strings.Contains(strings.ToLower(f.Type), "key") {
			hasPrivateKey = true
			break
		}
	}
	if !hasPrivateKey {
		t.Error("should detect private key type")
	}
}

// SC-6: Detects passwords in assignments.
func TestSecrets_Passwords(t *testing.T) {
	content := `DATABASE_PASSWORD = "super_secret_p@ssw0rd!"
db_password = "another_real_password"
`
	d := NewSecretDetector()
	findings := d.Detect(content)

	if len(findings) == 0 {
		t.Fatal("should detect password assignments")
	}

	hasPassword := false
	for _, f := range findings {
		if strings.Contains(strings.ToLower(f.Type), "password") {
			hasPassword = true
			break
		}
	}
	if !hasPassword {
		t.Error("should detect password type")
	}
}

// SC-6: Detects connection strings.
func TestSecrets_ConnectionStrings(t *testing.T) {
	content := `DB_URL = "postgresql://admin:secret@db.example.com:5432/production"
REDIS_URL = "redis://:p@ssword@redis.example.com:6379/0"
`
	d := NewSecretDetector()
	findings := d.Detect(content)

	if len(findings) == 0 {
		t.Fatal("should detect connection strings with credentials")
	}
}

// SC-6: Redacted content has no secret values.
func TestSecrets_Redaction(t *testing.T) {
	content := `config = {
    "api_key": "sk-proj-abc123def456ghi789",
    "aws_key": "AKIAIOSFODNN7EXAMPLE",
    "name": "my-app"
}
`
	d := NewSecretDetector()
	findings := d.Detect(content)
	redacted := d.Redact(content, findings)

	// Redacted content must not contain the secret values
	if strings.Contains(redacted, "sk-proj-abc123def456ghi789") {
		t.Error("redacted content still contains API key")
	}
	if strings.Contains(redacted, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("redacted content still contains AWS key")
	}
	// Should contain [REDACTED] markers
	if !strings.Contains(redacted, "[REDACTED]") {
		t.Error("redacted content should contain [REDACTED] markers")
	}
	// Non-secret content should be preserved
	if !strings.Contains(redacted, "my-app") {
		t.Error("redacted content should preserve non-secret values")
	}
}

// SC-6: HasSecrets flag set on affected chunks.
func TestSecrets_HasSecretsFlag(t *testing.T) {
	content := `API_KEY = "real-secret-key-value-here-12345"
`
	d := NewSecretDetector()
	findings := d.Detect(content)

	if len(findings) == 0 {
		t.Fatal("should detect at least one secret")
	}

	// The HasSecrets method on findings should indicate secrets were found
	if !d.HasSecrets(findings) {
		t.Error("HasSecrets should return true when findings exist")
	}

	// No secrets case
	cleanContent := `name = "my-app"
version = "1.0"
`
	cleanFindings := d.Detect(cleanContent)
	if d.HasSecrets(cleanFindings) {
		t.Error("HasSecrets should return false for clean content")
	}
}

// SC-6: Placeholder values do not trigger false positives.
func TestSecrets_PlaceholderAvoidance(t *testing.T) {
	content := `API_KEY = "your-api-key-here"
SECRET = "example-secret"
PASSWORD = "changeme"
TOKEN = "${GITHUB_TOKEN}"
API_SECRET = "{{secret_value}}"
DB_PASS = "your-password-here"
`
	d := NewSecretDetector()
	findings := d.Detect(content)

	if len(findings) != 0 {
		for _, f := range findings {
			t.Errorf("false positive: detected %s on line %d (match: %q)", f.Type, f.Line, f.Match)
		}
	}
}

// SC-6: Multiple secret types in same file all detected.
func TestSecrets_MultipleTypes(t *testing.T) {
	content := `AWS_ACCESS_KEY_ID = "AKIAIOSFODNN7EXAMPLE"
DATABASE_PASSWORD = "super_secret_password"
API_KEY = "sk-live-abc123def456ghi789jkl012"
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA2f3h3v4m
-----END RSA PRIVATE KEY-----
`
	d := NewSecretDetector()
	findings := d.Detect(content)

	// Should find multiple different types
	types := make(map[string]bool)
	for _, f := range findings {
		types[f.Type] = true
	}

	if len(types) < 2 {
		t.Errorf("should detect multiple secret types, got %d: %v", len(types), types)
	}
}

// SC-6: Findings include correct line numbers.
func TestSecrets_LineNumbers(t *testing.T) {
	content := `line one
line two
API_KEY = "sk-real-secret-key-12345678901234"
line four
`
	d := NewSecretDetector()
	findings := d.Detect(content)

	if len(findings) == 0 {
		t.Fatal("should detect API key")
	}

	// The finding should be on line 3 (where the key is)
	for _, f := range findings {
		if f.Line == 0 {
			t.Error("finding should have non-zero line number")
		}
		if f.Line != 3 {
			t.Errorf("finding line = %d, want 3", f.Line)
		}
	}
}
