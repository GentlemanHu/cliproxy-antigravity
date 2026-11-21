package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func containsModelID(models []modelInfo, id string) bool {
	for _, model := range models {
		if model.ID == id {
			return true
		}
	}
	return false
}

func callModelsForAuth(t *testing.T, home string) []modelInfo {
	t.Helper()
	request, err := json.Marshal(authModelRequest{
		AuthProvider: providerID,
		Metadata:     map[string]any{"type": providerID, "home": home},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := handleMethod(methodModelForAuth, request)
	if err != nil {
		t.Fatal(err)
	}
	var envelopeResponse envelope
	if err := json.Unmarshal(raw, &envelopeResponse); err != nil {
		t.Fatal(err)
	}
	if !envelopeResponse.OK {
		t.Fatalf("model.for_auth returned error: %s", raw)
	}
	var result modelResponse
	if err := json.Unmarshal(envelopeResponse.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Provider != providerID {
		t.Fatalf("provider = %q, want %q", result.Provider, providerID)
	}
	return result.Models
}

func TestOAuthModelScopeAndPerAuthDiscovery(t *testing.T) {
	if got := pluginRegistration().Capabilities.ExecutorModelScope; got != "oauth" {
		t.Fatalf("executor model scope = %q, expected oauth to enable per-auth aliases", got)
	}
	if runtime.GOOS == "windows" {
		t.Skip("fake agy shell executable requires POSIX")
	}

	root := t.TempDir()
	workHome := filepath.Join(root, "work")
	personalHome := filepath.Join(root, "personal")
	for _, home := range []string{workHome, personalHome} {
		if err := os.MkdirAll(home, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(root, "agy")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"models\" ]; then\n" +
		"  case \"$HOME\" in\n" +
		"    */work) printf 'account-work-only\\n';;\n" +
		"    */personal) printf 'account-personal-only\\n';;\n" +
		"  esac\n" +
		"fi\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	previous := currentConfig()
	t.Cleanup(func() { setConfig(previous) })
	cfg := defaultConfig()
	cfg.BinaryPath = binary
	setConfig(cfg)

	workModels := callModelsForAuth(t, workHome)
	personalModels := callModelsForAuth(t, personalHome)
	if !containsModelID(workModels, "agy/account-work-only") || containsModelID(workModels, "agy/account-personal-only") {
		t.Fatalf("work account model discovery leaked another account: %+v", workModels)
	}
	if !containsModelID(personalModels, "agy/account-personal-only") || containsModelID(personalModels, "agy/account-work-only") {
		t.Fatalf("personal account model discovery leaked another account: %+v", personalModels)
	}
	if !containsModelID(workModels, "gemini-3.8-flash-high") || !containsModelID(personalModels, "gemini-3.8-flash-high") {
		t.Fatalf("builtin source model must remain available for oauth-model-alias registration")
	}
	workCfg, err := withAccount(cfg, map[string]any{"home": workHome})
	if err != nil {
		t.Fatal(err)
	}
	personalCfg, err := withAccount(cfg, map[string]any{"home": personalHome})
	if err != nil {
		t.Fatal(err)
	}
	if got := resolveNativeModel("agy/account-work-only", workCfg); got != "account-work-only" {
		t.Fatalf("work account native model = %q", got)
	}
	if got := resolveNativeModel("agy/account-personal-only", personalCfg); got != "account-personal-only" {
		t.Fatalf("personal account native model = %q", got)
	}
}

func TestModelForAuthRejectsMissingHome(t *testing.T) {
	raw, _ := json.Marshal(authModelRequest{
		AuthProvider: providerID,
		Metadata:     map[string]any{"home": filepath.Join(t.TempDir(), "not-here")},
	})
	_, err := handleMethod(methodModelForAuth, raw)
	if pe := asPluginError(err); pe == nil || pe.Code != "agy_account_home_missing" {
		t.Fatalf("expected explicit missing-account error, got %v", err)
	}
}

func TestOAuthRouterDefersToCredentialProvider(t *testing.T) {
	if routed := routeModel(modelRouteRequest{
		RequestedModel:     "agy/gemini-3.8-flash-high",
		AvailableProviders: []string{"agy"},
	}); routed.Handled {
		t.Fatalf("OAuth-backed agy requests must use normal credential scheduling")
	}
	if routed := routeModel(modelRouteRequest{
		RequestedModel:     "claude-opus-4-7-anti",
		AvailableProviders: []string{"agy"},
	}); routed.Handled {
		t.Fatalf("OAuth aliases must stay with CLIProxyAPI's resolved provider")
	}
}

func TestOptionalMetadataStrippingIsScopedToSystemInstructions(t *testing.T) {
	body := []byte(`{
		"model":"agy/default",
		"messages":[
			{"role":"system","content":"You are a Claude agent, built on Anthropic's Claude Agent SDK.\nx-anthropic-billing-header: cc_version=1\nanthropic-beta: test\nBe accurate."},
			{"role":"user","content":"Please explain the literal x-anthropic-billing-header: example."}
		]
	}`)
	plain, err := buildPrompt(body, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain, "cc_version=1") {
		t.Fatal("default behavior should leave system text untouched")
	}
	filtered, err := buildPromptWithOptions(body, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(filtered, "cc_version=1") || strings.Contains(filtered, "anthropic-beta: test") {
		t.Fatalf("transport metadata still in system text: %q", filtered)
	}
	if !strings.Contains(filtered, "You are a Claude agent") || !strings.Contains(filtered, "Be accurate.") {
		t.Fatal("sanitizer must preserve valid agent instructions")
	}
	if !strings.Contains(filtered, "Please explain the literal x-anthropic-billing-header: example.") {
		t.Fatal("sanitizer must preserve all user content")
	}
}

func TestStripClientMetadataConfig(t *testing.T) {
	cfg, err := parsePluginConfig([]byte("strip_client_metadata: true\n"))
	if err != nil || !cfg.StripClientMetadata {
		t.Fatalf("strip_client_metadata parse: cfg=%+v err=%v", cfg, err)
	}
	if _, err := parsePluginConfig([]byte("strip_client_metadata: not-a-bool\n")); err == nil {
		t.Fatal("invalid strip_client_metadata should be rejected")
	}
}
