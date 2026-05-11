package email

import (
	"strings"
	"testing"
)

func TestRenderConfirmation(t *testing.T) {
	msg := RenderConfirmation("golang/go", "http://localhost:8080/api/confirm/abc")

	if !strings.Contains(msg.Subject, "golang/go") {
		t.Errorf("subject does not mention repo: %q", msg.Subject)
	}
	if !strings.Contains(msg.HTMLBody, "http://localhost:8080/api/confirm/abc") {
		t.Errorf("body missing confirm URL: %q", msg.HTMLBody)
	}
	if !strings.Contains(msg.HTMLBody, "<html>") || !strings.Contains(msg.HTMLBody, "</html>") {
		t.Errorf("body is not a complete HTML document")
	}
}

func TestRenderReleaseNotification(t *testing.T) {
	msg := RenderReleaseNotification(
		"golang/go", "v1.22.0",
		"https://github.com/golang/go/releases/tag/v1.22.0",
		"http://localhost:8080/api/unsubscribe/xyz",
	)

	if !strings.Contains(msg.Subject, "v1.22.0") || !strings.Contains(msg.Subject, "golang/go") {
		t.Errorf("subject missing repo or tag: %q", msg.Subject)
	}
	if !strings.Contains(msg.HTMLBody, "https://github.com/golang/go/releases/tag/v1.22.0") {
		t.Errorf("body missing release URL")
	}
	if !strings.Contains(msg.HTMLBody, "http://localhost:8080/api/unsubscribe/xyz") {
		t.Errorf("body missing unsubscribe URL")
	}
}
