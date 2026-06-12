package sms

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubServer wires up an httptest.Server with the canned response and
// returns a TwilioSender pointed at it. The caller is responsible for
// stopping the server via t.Cleanup.
func stubServer(t *testing.T, status int, body string) *TwilioSender {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	s, err := NewTwilio(TwilioConfig{
		AccountSID: "ACtest",
		AuthToken:  "tok",
		From:       "+15005550006",
		BaseURL:    srv.URL,
	})
	if err != nil {
		t.Fatalf("NewTwilio: %v", err)
	}
	return s
}

func TestNewTwilioRejectsMissingCredentials(t *testing.T) {
	if _, err := NewTwilio(TwilioConfig{From: "+15005550006"}); err == nil {
		t.Errorf("expected error when account_sid/auth_token missing")
	}
}

func TestNewTwilioRejectsMissingSender(t *testing.T) {
	if _, err := NewTwilio(TwilioConfig{AccountSID: "AC", AuthToken: "tok"}); err == nil {
		t.Errorf("expected error when both From and MessagingServiceSID empty")
	}
}

func TestTwilioSendHappyPath(t *testing.T) {
	s := stubServer(t, http.StatusCreated, `{"sid":"SMabc123","status":"queued"}`)
	r, err := s.Send(context.Background(), Message{To: "+15555551212", Body: "hi"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if r.ProviderID != "SMabc123" || r.Status != "queued" {
		t.Errorf("unexpected receipt: %+v", r)
	}
}

func TestTwilioSendFormatsRequest(t *testing.T) {
	var (
		gotBody  string
		gotPath  string
		gotAuth  string
		gotCType string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"SM1","status":"queued"}`))
	}))
	t.Cleanup(srv.Close)

	s, _ := NewTwilio(TwilioConfig{
		AccountSID: "AC123",
		AuthToken:  "tok",
		From:       "+15005550006",
		BaseURL:    srv.URL,
	})
	if _, err := s.Send(context.Background(), Message{To: "+15555551212", Body: "hello world"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(gotPath, "/2010-04-01/Accounts/AC123/Messages.json") {
		t.Errorf("path = %q, want Messages.json under AC123", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Errorf("Authorization header missing Basic prefix: %q", gotAuth)
	}
	if gotCType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", gotCType)
	}
	for _, want := range []string{"To=%2B15555551212", "From=%2B15005550006", "Body=hello+world"} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("body missing %q, got %q", want, gotBody)
		}
	}
}

func TestTwilioSendUsesMessagingServiceWhenSet(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sid":"SM1","status":"queued"}`))
	}))
	t.Cleanup(srv.Close)

	s, _ := NewTwilio(TwilioConfig{
		AccountSID:          "AC1",
		AuthToken:           "tok",
		MessagingServiceSID: "MGsvc",
		BaseURL:             srv.URL,
	})
	if _, err := s.Send(context.Background(), Message{To: "+15555551212", Body: "hi"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(gotBody, "MessagingServiceSid=MGsvc") {
		t.Errorf("body should set MessagingServiceSid, got %q", gotBody)
	}
	if strings.Contains(gotBody, "From=") {
		t.Errorf("body should NOT set From when MessagingServiceSid is in use, got %q", gotBody)
	}
}

func TestTwilioSendInvalidRecipient(t *testing.T) {
	s := stubServer(t, http.StatusBadRequest, `{"code":21211,"message":"Invalid 'To' Phone Number"}`)
	_, err := s.Send(context.Background(), Message{To: "not-a-number", Body: "hi"})
	if !errors.Is(err, ErrInvalidRecipient) {
		t.Errorf("expected ErrInvalidRecipient, got %v", err)
	}
}

func TestTwilioSendUnregisteredFromNumber(t *testing.T) {
	s := stubServer(t, http.StatusBadRequest, `{"code":21659,"message":"From Phone Number not Twilio number"}`)
	_, err := s.Send(context.Background(), Message{To: "+15555551212", Body: "hi"})
	if !errors.Is(err, ErrUnregisteredFromNumber) {
		t.Errorf("expected ErrUnregisteredFromNumber, got %v", err)
	}
}

func TestTwilioSendCarrierBlocked(t *testing.T) {
	s := stubServer(t, http.StatusBadRequest, `{"code":30034,"message":"US 10DLC - Number not registered"}`)
	_, err := s.Send(context.Background(), Message{To: "+15555551212", Body: "hi"})
	if !errors.Is(err, ErrCarrierBlocked) {
		t.Errorf("expected ErrCarrierBlocked, got %v", err)
	}
}

func TestTwilioSendUnknownCodeIsRetryable(t *testing.T) {
	s := stubServer(t, http.StatusBadRequest, `{"code":99999,"message":"new error class","more_info":"https://x"}`)
	_, err := s.Send(context.Background(), Message{To: "+15555551212", Body: "hi"})
	if err == nil {
		t.Fatalf("expected error on non-2xx response")
	}
	for _, sentinel := range []error{ErrInvalidRecipient, ErrUnregisteredFromNumber, ErrCarrierBlocked, ErrRefused} {
		if errors.Is(err, sentinel) {
			t.Errorf("unknown code %d should not map to sentinel %v", 99999, sentinel)
		}
	}
}

func TestTwilioSendEmptyTo(t *testing.T) {
	s := stubServer(t, http.StatusCreated, `{}`)
	_, err := s.Send(context.Background(), Message{To: "", Body: "hi"})
	if !errors.Is(err, ErrInvalidRecipient) {
		t.Errorf("expected ErrInvalidRecipient on empty To, got %v", err)
	}
}
