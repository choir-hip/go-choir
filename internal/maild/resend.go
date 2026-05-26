package maild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type resendClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type resendReceivedEmail struct {
	ID          string                 `json:"id"`
	To          []string               `json:"to"`
	From        string                 `json:"from"`
	CreatedAt   string                 `json:"created_at"`
	Subject     string                 `json:"subject"`
	Text        string                 `json:"text"`
	HTML        string                 `json:"html"`
	Headers     map[string]string      `json:"headers"`
	Bcc         []string               `json:"bcc"`
	Cc          []string               `json:"cc"`
	ReplyTo     []string               `json:"reply_to"`
	MessageID   string                 `json:"message_id"`
	Raw         *resendRawDownload     `json:"raw"`
	Attachments []resendAttachmentMeta `json:"attachments"`
}

type resendRawDownload struct {
	DownloadURL string `json:"download_url"`
	ExpiresAt   string `json:"expires_at"`
}

type resendAttachmentMeta struct {
	ID                 string `json:"id"`
	Filename           string `json:"filename"`
	ContentType        string `json:"content_type"`
	ContentDisposition string `json:"content_disposition"`
	ContentID          string `json:"content_id"`
	Size               int64  `json:"size"`
}

type resendSendRequest struct {
	From    string         `json:"from"`
	To      []string       `json:"to"`
	Cc      []string       `json:"cc,omitempty"`
	Bcc     []string       `json:"bcc,omitempty"`
	Subject string         `json:"subject"`
	Text    string         `json:"text,omitempty"`
	HTML    string         `json:"html,omitempty"`
	Headers map[string]any `json:"headers,omitempty"`
}

type resendSendResponse struct {
	ID string `json:"id"`
}

func newResendClient(cfg *Config, client *http.Client) resendClient {
	if client == nil {
		client = http.DefaultClient
	}
	return resendClient{
		baseURL: strings.TrimRight(cfg.ResendBaseURL, "/"),
		apiKey:  cfg.ResendAPIKey,
		client:  client,
	}
}

func (c resendClient) retrieveReceivedEmail(ctx context.Context, emailID string) (resendReceivedEmail, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return resendReceivedEmail{}, fmt.Errorf("RESEND_API_KEY is not configured")
	}
	if strings.TrimSpace(emailID) == "" {
		return resendReceivedEmail{}, fmt.Errorf("email id is required")
	}
	endpoint := c.baseURL + "/emails/receiving/" + url.PathEscape(emailID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return resendReceivedEmail{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return resendReceivedEmail{}, fmt.Errorf("retrieve received email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return resendReceivedEmail{}, fmt.Errorf("retrieve received email status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var email resendReceivedEmail
	if err := json.NewDecoder(resp.Body).Decode(&email); err != nil {
		return resendReceivedEmail{}, fmt.Errorf("decode received email: %w", err)
	}
	return email, nil
}

func (c resendClient) sendEmail(ctx context.Context, payload resendSendRequest) (resendSendResponse, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return resendSendResponse{}, fmt.Errorf("RESEND_API_KEY is not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return resendSendResponse{}, fmt.Errorf("marshal send email: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return resendSendResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return resendSendResponse{}, fmt.Errorf("send email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return resendSendResponse{}, fmt.Errorf("send email status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out resendSendResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return resendSendResponse{}, fmt.Errorf("decode send email: %w", err)
	}
	if strings.TrimSpace(out.ID) == "" {
		return resendSendResponse{}, fmt.Errorf("send email response missing id")
	}
	return out, nil
}
