package pii

import (
	"strings"
	"testing"
)

// All PII in this file is synthetic. No real personal data is used.
// Emails, phones, SSNs, and cards are structurally valid but fictitious.

func TestRegexRedactor_Email(t *testing.T) {
	r := NewRegexRedactor()
	in := "contact alice.example@example.com for details"
	out, findings, err := r.RedactTextWithLuhn(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if !strings.Contains(out, RedactionToken(ClassEmail)) {
		t.Fatalf("expected email token, got %q", out)
	}
	if strings.Contains(out, "alice.example@example.com") {
		t.Fatalf("raw email leaked: %q", out)
	}
	if len(findings) != 1 || findings[0].Class != ClassEmail {
		t.Fatalf("expected 1 email finding, got %+v", findings)
	}
}

func TestRegexRedactor_Phone(t *testing.T) {
	r := NewRegexRedactor()
	cases := []string{
		"call me at +1 555 123 4567 today",
		"phone: (555) 123-4567",
		"tel 555.123.4567",
	}
	for _, in := range cases {
		out, findings, err := r.RedactTextWithLuhn(in)
		if err != nil {
			t.Fatalf("redact %q: %v", in, err)
		}
		if len(findings) == 0 {
			t.Fatalf("expected phone finding for %q, got %q", in, out)
		}
		if !strings.Contains(out, RedactionToken(ClassPhone)) {
			t.Fatalf("expected phone token for %q, got %q", in, out)
		}
	}
}

func TestRegexRedactor_SSN(t *testing.T) {
	r := NewRegexRedactor()
	in := "ssn 123-45-6789 on file"
	out, findings, err := r.RedactTextWithLuhn(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if len(findings) != 1 || findings[0].Class != ClassSSN {
		t.Fatalf("expected 1 ssn finding, got %+v", findings)
	}
	if strings.Contains(out, "123-45-6789") {
		t.Fatalf("raw ssn leaked: %q", out)
	}
}

func TestRegexRedactor_CreditCard_LuhnFiltered(t *testing.T) {
	r := NewRegexRedactor()
	// 4242 4242 4242 4242 is a canonical test card (Luhn-valid).
	in := "card 4242 4242 4242 4242 expires 12/30"
	out, findings, err := r.RedactTextWithLuhn(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if len(findings) != 1 || findings[0].Class != ClassCreditCard {
		t.Fatalf("expected 1 credit_card finding, got %+v", findings)
	}
	if strings.Contains(out, "4242 4242 4242 4242") {
		t.Fatalf("raw card leaked: %q", out)
	}
	// A 16-digit run that fails Luhn must NOT be redacted as a card.
	badIn := "ref 1234 5678 9012 3456 done"
	badOut, badFindings, err := r.RedactTextWithLuhn(badIn)
	if err != nil {
		t.Fatalf("redact bad: %v", err)
	}
	for _, f := range badFindings {
		if f.Class == ClassCreditCard {
			t.Fatalf("Luhn-invalid run redacted as card: %q -> %q (%+v)", badIn, badOut, f)
		}
	}
}

func TestRegexRedactor_APIKey(t *testing.T) {
	r := NewRegexRedactor()
	cases := []string{
		"key sk-abcdefghijklmnopqrstuvwxyz0123456789",
		"api_key=AKIATESTKEY1234567XYZ",
		"Authorization: Bearer dGhpcyBpcyBhIHRlc3QgdG9rZW4",
		"AKIATESTKEY12345678AB", // AWS-style access key id: AKIA + 16 uppercase
	}
	for _, in := range cases {
		out, findings, err := r.RedactTextWithLuhn(in)
		if err != nil {
			t.Fatalf("redact %q: %v", in, err)
		}
		if len(findings) == 0 {
			t.Fatalf("expected api_key finding for %q, got %q", in, out)
		}
	}
}

func TestRegexRedactor_IPv4(t *testing.T) {
	r := NewRegexRedactor()
	in := "ssh from 192.168.1.23 to 10.0.0.1"
	out, findings, err := r.RedactTextWithLuhn(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 ip findings, got %+v", findings)
	}
	if strings.Contains(out, "192.168.1.23") || strings.Contains(out, "10.0.0.1") {
		t.Fatalf("raw ip leaked: %q", out)
	}
}

func TestRegexRedactor_Credential(t *testing.T) {
	r := NewRegexRedactor()
	in := "password=hunter2letmein"
	out, findings, err := r.RedactTextWithLuhn(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if len(findings) == 0 {
		t.Fatalf("expected credential finding, got %q", out)
	}
	if strings.Contains(out, "hunter2letmein") {
		t.Fatalf("raw password leaked: %q", out)
	}
}

func TestRegexRedactor_NoPIIPassesThrough(t *testing.T) {
	r := NewRegexRedactor()
	in := "the agent completed run abc-123 with trajectory t-456"
	out, findings, err := r.RedactTextWithLuhn(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	if out != in {
		t.Fatalf("non-PII text changed: %q -> %q", in, out)
	}
}

func TestRegexRedactor_MultipleClassesReal(t *testing.T) {
	r := NewRegexRedactor()
	in := "email alice@example.com phone +1 555 123 4567 ip 203.0.113.7"
	out, findings, err := r.RedactTextWithLuhn(in)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	classes := map[PIIClass]bool{}
	for _, f := range findings {
		classes[f.Class] = true
	}
	if !classes[ClassEmail] || !classes[ClassPhone] || !classes[ClassIP] {
		t.Fatalf("expected email+phone+ip, got %+v (out=%q)", findings, out)
	}
	for _, raw := range []string{"alice@example.com", "+1 555 123 4567", "203.0.113.7"} {
		if strings.Contains(out, raw) {
			t.Fatalf("raw PII leaked %q in %q", raw, out)
		}
	}
}

func TestRedactionToken(t *testing.T) {
	if got := RedactionToken(ClassEmail); got != "[REDACTED:email]" {
		t.Fatalf("token = %q", got)
	}
	if got := RedactionToken(ClassCreditCard); got != "[REDACTED:credit_card]" {
		t.Fatalf("token = %q", got)
	}
}

func TestSortFindings_DropsOverlaps(t *testing.T) {
	in := []Finding{
		{Class: ClassEmail, Start: 0, End: 20},
		{Class: ClassPhone, Start: 5, End: 25}, // overlaps, dropped
		{Class: ClassIP, Start: 30, End: 40},
	}
	got := sortFindings(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 non-overlapping, got %+v", got)
	}
	if got[0].Start != 0 || got[1].Start != 30 {
		t.Fatalf("wrong order: %+v", got)
	}
}
