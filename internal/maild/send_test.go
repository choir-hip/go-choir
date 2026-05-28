package maild

import (
	"strings"
	"testing"
)

func TestBuildResendSendRequestGeneratesSafeHTMLPart(t *testing.T) {
	payload, err := buildResendSendRequest(sendEmailRequest{
		ToAddresses: []string{"friend@example.com"},
		Subject:     "Hello",
		TextBody:    "Intro paragraph.\n\n## Section\n\n- First\n- <second>",
	}, EmailAlias{Domain: "choir.news", LocalPart: "000"})
	if err != nil {
		t.Fatalf("buildResendSendRequest: %v", err)
	}
	if payload.Text != "Intro paragraph.\n\n## Section\n\n- First\n- <second>" {
		t.Fatalf("payload text = %q", payload.Text)
	}
	if !strings.Contains(payload.HTML, "<h2") || !strings.Contains(payload.HTML, "<li") {
		t.Fatalf("payload HTML did not render markdown structure: %q", payload.HTML)
	}
	if strings.Contains(payload.HTML, "<second>") || !strings.Contains(payload.HTML, "&lt;second&gt;") {
		t.Fatalf("payload HTML did not escape text: %q", payload.HTML)
	}
}
