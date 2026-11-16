package slidecaptcha_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/captcha"
	"github.com/abysstract/abysstract-admin/backend/captcha/slidecaptcha"
)

func TestChallengeAndVerify(t *testing.T) {
	v, err := captcha.Open(captcha.Config{Provider: "slide"})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := v.(captcha.Challenger).Challenge(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var p slidecaptcha.Payload
	if err := json.Unmarshal([]byte(ch.Payload), &p); err != nil {
		t.Fatal(err)
	}
	if ch.Kind != "slide" || !strings.HasPrefix(p.Image, "data:image/jpeg;base64,") || !strings.HasPrefix(p.Thumb, "data:image/png;base64,") {
		t.Fatalf("payload kind=%q image=%.30q thumb=%.30q", ch.Kind, p.Image, p.Thumb)
	}
	if strings.Contains(ch.Payload, `"x"`) {
		t.Fatal("payload must not reveal the answer")
	}
	var ans struct{ X int }
	if err := json.Unmarshal(ch.Answer, &ans); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, tc := range []struct {
		x  int
		ok bool
	}{{ans.X, true}, {ans.X + slidecaptcha.Tolerance, true}, {ans.X - slidecaptcha.Tolerance - 1, false}, {ans.X + 40, false}} {
		err := v.Verify(ctx, captcha.Submission{Token: fmt.Sprintf(`{"x":%d}`, tc.x), Answer: ch.Answer})
		if tc.ok != (err == nil) {
			t.Fatalf("x=%d (answer %d): err=%v", tc.x, ans.X, err)
		}
		if err != nil && !errors.Is(err, captcha.ErrInvalid) {
			t.Fatalf("x=%d: want ErrInvalid, got %v", tc.x, err)
		}
	}
	for _, token := range []string{"", "nope", `{"y":1}`} {
		if err := v.Verify(ctx, captcha.Submission{Token: token, Answer: ch.Answer}); !errors.Is(err, captcha.ErrInvalid) {
			t.Fatalf("token %q: %v", token, err)
		}
	}
}
