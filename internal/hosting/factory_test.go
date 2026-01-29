package hosting

import (
	"testing"
)

func TestResolveProviderType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider string
		wantType ProviderType
		wantErr  bool
	}{
		{
			name:     "explicit github",
			provider: "github",
			wantType: ProviderGitHub,
		},
		{
			name:     "explicit gitlab",
			provider: "gitlab",
			wantType: ProviderGitLab,
		},
		{
			name:    "unknown provider returns error",
			provider: "bitbucket",
			wantErr: true,
		},
		{
			name:    "unknown provider: azure",
			provider: "azure",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := Config{Provider: tt.provider}
			// resolveProviderType with explicit provider doesn't need a real workDir
			got, err := resolveProviderType("", cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveProviderType() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.wantType {
				t.Errorf("resolveProviderType() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestResolveProviderType_AutoRequiresGitRepo(t *testing.T) {
	t.Parallel()

	// "auto" with a non-existent workDir should fail because git can't get the remote.
	cfg := Config{Provider: "auto"}
	_, err := resolveProviderType("/nonexistent/path", cfg)
	if err == nil {
		t.Fatal("resolveProviderType() with auto and invalid workDir should return error")
	}
}

func TestResolveProviderType_EmptyProviderIsAuto(t *testing.T) {
	t.Parallel()

	// Empty provider is treated as "auto", which requires a real git repo.
	// Without one, it should fail.
	cfg := Config{Provider: ""}
	_, err := resolveProviderType("/nonexistent/path", cfg)
	if err == nil {
		t.Fatal("resolveProviderType() with empty provider and invalid workDir should return error")
	}
}

func TestNewProvider_UnregisteredProvider(t *testing.T) {
	t.Parallel()

	// Even with a valid explicit provider type, if no constructor is registered
	// for it, NewProvider should fail. We test this by using a valid type string
	// but temporarily removing its constructor. Since that would affect global
	// state, instead we test the error path by checking that unknown providers
	// produce errors through the resolveProviderType path.
	cfg := Config{Provider: "bitbucket"}
	_, err := NewProvider("", cfg)
	if err == nil {
		t.Fatal("NewProvider() with unknown provider should return error")
	}
}

func TestRegisteredProviders(t *testing.T) {
	t.Parallel()

	providers := registeredProviders()
	// registeredProviders returns whatever is currently registered.
	// We can't assert specific providers here since the github/gitlab
	// init() functions may or may not have run depending on imports.
	// Just verify it returns without panicking and returns a slice.
	if providers == nil {
		// nil is fine — it means no providers are registered in this test binary
		// since we're in the hosting package, not importing github/gitlab.
		return
	}
}
