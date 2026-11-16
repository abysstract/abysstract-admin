package siteverify_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/captcha"
	"github.com/abysstract/abysstract-admin/backend/captcha/siteverify"
)

func TestVerify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.PostForm.Get("secret") != "s3cret" || r.PostForm.Get("remoteip") != "198.51.100.7" {
			t.Errorf("form = %v", r.PostForm)
		}
		switch r.PostForm.Get("response") {
		case "good":
			_, _ = w.Write([]byte(`{"success":true}`))
		case "bad-secret":
			_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-secret"]}`))
		default:
			_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
		}
	}))
	defer srv.Close()
	v, err := siteverify.New("turnstile", srv.URL, captcha.Config{Secret: "s3cret", SiteKey: "site", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sub := func(token string) captcha.Submission {
		return captcha.Submission{Token: token, RemoteIP: "198.51.100.7"}
	}
	if err := v.Verify(ctx, sub("good")); err != nil {
		t.Fatalf("good: %v", err)
	}
	if err := v.Verify(ctx, sub("wrong")); !errors.Is(err, captcha.ErrInvalid) {
		t.Fatalf("wrong: %v", err)
	}
	if err := v.Verify(ctx, sub("bad-secret")); err == nil || errors.Is(err, captcha.ErrInvalid) {
		t.Fatalf("bad secret must be an infrastructure error: %v", err)
	}
	ch, err := v.Challenge(ctx)
	if err != nil || ch.Kind != "turnstile" || ch.Payload != `{"siteKey":"site"}` || ch.Answer != nil {
		t.Fatalf("challenge = %+v, %v", ch, err)
	}
}

func TestRegistered(t *testing.T) {
	for _, name := range []string{"turnstile", "hcaptcha"} {
		if !captcha.Registered(name) {
			t.Fatalf("%s not registered", name)
		}
		if _, err := captcha.Open(captcha.Config{Provider: name}); err == nil {
			t.Fatalf("%s without secret should fail", name)
		}
	}
}
