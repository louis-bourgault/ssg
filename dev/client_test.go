package dev

import (
	"strings"
	"testing"
)

func TestInjectClientScriptUsesHTMLTokenizer(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"uppercase", `<!DOCTYPE html><HTML><HEAD><title>x</title></HEAD><BODY>x</BODY></HTML>`},
		{"attributes", `<html><head data-theme="dark"><title>x</title></head></html>`},
		{"no head", `<main>fragment</main>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := injectClientScript(test.input)
			if strings.Count(got, clientTag) != 1 {
				t.Fatalf("injection count was not one: %s", got)
			}
			if !strings.Contains(got, "x") && !strings.Contains(got, "fragment") {
				t.Fatalf("document content was lost: %s", got)
			}
		})
	}
}

func TestLiveReloadClientUsesPageLocationAndReconnects(t *testing.T) {
	checks := []string{
		`location.protocol === "https:" ? "wss:" : "ws:"`,
		`location.host`,
		`/__ssg/ws`,
		`scheduleReconnect`,
		`JSON.parse`,
		`clear-error`,
		`refreshCSS`,
	}
	for _, check := range checks {
		if !strings.Contains(liveReloadClient, check) {
			t.Errorf("client script does not contain %q", check)
		}
	}
	if strings.Contains(liveReloadClient, "localhost:8080") {
		t.Fatal("client script hardcodes localhost:8080")
	}
}
