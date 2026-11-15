// Package slidecaptcha is a self-hosted slider puzzle built on go-captcha
// (github.com/wenlng/go-captcha). Importing it registers the captcha provider
// "slide"; it needs no external service, so it works on intranets.
//
// The challenge is a background image with a cut-out and the matching piece.
// The client drags the piece horizontally and submits {"x": <left edge>};
// Verify accepts it within Tolerance pixels of the cut-out.
//
// The package embeds the go-captcha-assets images (a few MB in the binary);
// applications that do not use the slider leave the import out.
package slidecaptcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/wenlng/go-captcha-assets/resources/images"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/slide"

	"github.com/abysstract/abysstract-admin/backend/captcha"
)

// Kind is Challenge.Kind of slider challenges; @abysstract/admin-core
// renders it without extra configuration.
const Kind = "slide"

// Tolerance is how many pixels the piece may be off horizontally.
const Tolerance = 6

// Size of the background image in pixels.
const (
	Width  = 300
	Height = 220
)

func init() {
	captcha.Register(Kind, func(captcha.Config) (captcha.Verifier, error) { return New() })
}

// Payload is the JSON sent to the client.
type Payload struct {
	Image       string `json:"image"` // background, JPEG data URL
	Thumb       string `json:"thumb"` // piece, PNG data URL
	ThumbX      int    `json:"thumbX"`
	ThumbY      int    `json:"thumbY"`
	ThumbWidth  int    `json:"thumbWidth"`
	ThumbHeight int    `json:"thumbHeight"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type answer struct {
	X int `json:"x"`
}

// Verifier issues and checks slider challenges.
type Verifier struct {
	mu   sync.Mutex
	capt slide.Captcha
}

var (
	loadOnce  sync.Once
	loadErr   error
	resources []slide.Resource
)

func load() ([]slide.Resource, error) {
	loadOnce.Do(func() {
		backgrounds, err := images.GetImages()
		if err != nil {
			loadErr = fmt.Errorf("slidecaptcha: load backgrounds: %w", err)
			return
		}
		pieces, err := tiles.GetTiles()
		if err != nil {
			loadErr = fmt.Errorf("slidecaptcha: load tiles: %w", err)
			return
		}
		graphs := make([]*slide.GraphImage, 0, len(pieces))
		for _, p := range pieces {
			graphs = append(graphs, &slide.GraphImage{OverlayImage: p.OverlayImage, ShadowImage: p.ShadowImage, MaskImage: p.MaskImage})
		}
		resources = []slide.Resource{slide.WithBackgrounds(backgrounds), slide.WithGraphImages(graphs)}
	})
	return resources, loadErr
}

// New returns a slider Verifier using the bundled images.
func New() (*Verifier, error) {
	res, err := load()
	if err != nil {
		return nil, err
	}
	builder := slide.NewBuilder(slide.WithGenGraphNumber(1), slide.WithEnableGraphVerticalRandom(true))
	builder.SetResources(res...)
	return &Verifier{capt: builder.Make()}, nil
}

// Challenge draws a new puzzle.
func (v *Verifier) Challenge(context.Context) (captcha.Challenge, error) {
	v.mu.Lock()
	data, err := v.capt.Generate()
	v.mu.Unlock()
	if err != nil {
		return captcha.Challenge{}, fmt.Errorf("slidecaptcha: generate: %w", err)
	}
	block := data.GetData()
	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return captcha.Challenge{}, err
	}
	thumb, err := data.GetTileImage().ToBase64()
	if err != nil {
		return captcha.Challenge{}, err
	}
	payload, err := json.Marshal(Payload{
		Image: master, Thumb: thumb,
		ThumbX: block.DX, ThumbY: block.DY,
		ThumbWidth: block.Width, ThumbHeight: block.Height,
		Width: Width, Height: Height,
	})
	if err != nil {
		return captcha.Challenge{}, err
	}
	ans, err := json.Marshal(answer{X: block.X})
	if err != nil {
		return captcha.Challenge{}, err
	}
	return captcha.Challenge{Kind: Kind, Payload: string(payload), Answer: ans}, nil
}

// Verify checks the submitted {"x": n} against the cut-out position.
func (v *Verifier) Verify(_ context.Context, sub captcha.Submission) error {
	var want answer
	if err := json.Unmarshal(sub.Answer, &want); err != nil {
		return errors.Join(captcha.ErrInvalid, err)
	}
	var got struct {
		X *float64 `json:"x"`
	}
	if err := json.Unmarshal([]byte(sub.Token), &got); err != nil || got.X == nil {
		return captcha.ErrInvalid
	}
	diff := *got.X - float64(want.X)
	if diff < -Tolerance || diff > Tolerance {
		return captcha.ErrInvalid
	}
	return nil
}
