package slidecaptcha_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
	"testing/fstest"

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

func TestChallengeKeepsCutOutClearOfStart(t *testing.T) {
	v, err := slidecaptcha.New()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		ch, err := v.Challenge(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var p slidecaptcha.Payload
		if err := json.Unmarshal([]byte(ch.Payload), &p); err != nil {
			t.Fatal(err)
		}
		var ans struct{ X int }
		if err := json.Unmarshal(ch.Answer, &ans); err != nil {
			t.Fatal(err)
		}
		if p.Width != slidecaptcha.Width || p.Height != slidecaptcha.Height || p.ThumbX != 0 {
			t.Fatalf("payload size %dx%d start %d", p.Width, p.Height, p.ThumbX)
		}
		if ans.X < p.ThumbWidth || ans.X+p.ThumbWidth > p.Width {
			t.Fatalf("cut-out x=%d overlaps the start or leaves the image", ans.X)
		}
		if p.ThumbY < 0 || p.ThumbY+p.ThumbHeight > p.Height {
			t.Fatalf("cut-out y=%d leaves the image", p.ThumbY)
		}
	}
}

func TestWithBackgroundsAndLoadBackgrounds(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 800, 300))
	for i := range src.Pix {
		src.Pix[i] = byte(i)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"bg/a.png": {Data: buf.Bytes()}, "bg/readme.txt": {Data: []byte("x")}}
	images, err := slidecaptcha.LoadBackgrounds(fsys, "bg/*.png")
	if err != nil || len(images) != 1 {
		t.Fatalf("LoadBackgrounds: %d images, %v", len(images), err)
	}
	if _, err := slidecaptcha.LoadBackgrounds(fsys, "none/*.png"); err == nil {
		t.Fatal("want an error when nothing matches")
	}
	v, err := slidecaptcha.New(slidecaptcha.WithBackgrounds(images...))
	if err != nil {
		t.Fatal(err)
	}
	ch, err := v.Challenge(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var p slidecaptcha.Payload
	if err := json.Unmarshal([]byte(ch.Payload), &p); err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(p.Image, "data:image/jpeg;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != slidecaptcha.Width || cfg.Height != slidecaptcha.Height {
		t.Fatalf("background %dx%d, want %dx%d", cfg.Width, cfg.Height, slidecaptcha.Width, slidecaptcha.Height)
	}
}
