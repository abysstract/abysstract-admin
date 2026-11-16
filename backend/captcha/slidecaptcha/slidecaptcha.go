// Package slidecaptcha is a self-hosted slider puzzle. Importing it registers
// the captcha provider "slide"; it needs no external service, so it works on
// intranets.
//
// The challenge is a background image with a cut-out and the matching piece.
// The client drags the piece horizontally and submits {"x": <left edge>};
// Verify accepts it within Tolerance pixels of the cut-out.
//
// By default every background is drawn on the fly: soft, neutral shapes from a
// random seed, so the package embeds no images. Applications can supply their
// own pictures with WithBackgrounds and pass the Verifier to
// adminapp.Options.Captcha.
package slidecaptcha

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io/fs"
	"math"
	"math/rand/v2"
	"path"

	_ "image/gif" // decoder for LoadBackgrounds

	xdraw "golang.org/x/image/draw"

	"github.com/abysstract/abysstract-admin/backend/captcha"
)

// Kind is Challenge.Kind of slider challenges; @abysstract/admin-core
// renders it without extra configuration.
const Kind = "slide"

// Tolerance is how many pixels the piece may be off horizontally.
const Tolerance = 6

// Size of the background image in pixels. The client scales it to the width
// of the login form, so the 2:1 ratio is what matters.
const (
	Width  = 400
	Height = 200
)

// Size of the piece, including the round tab on its top edge.
const (
	pieceSize = 56
	tabRadius = 9
	margin    = 12
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

// Verifier issues and checks slider challenges. It is safe for concurrent use.
type Verifier struct {
	backgrounds []*image.RGBA
}

// Option configures New.
type Option func(*Verifier)

// WithBackgrounds draws challenges on the given pictures instead of generated
// ones. Each picture is scaled and cropped to fill Width×Height; pictures
// with fine, varied detail make the cut-out harder to find by machine.
func WithBackgrounds(images ...image.Image) Option {
	return func(v *Verifier) {
		for _, img := range images {
			v.backgrounds = append(v.backgrounds, cover(img))
		}
	}
}

// LoadBackgrounds decodes the JPEG, PNG and GIF files in fsys that match
// pattern (path.Match syntax, for example "login/*.jpg"), for use with
// WithBackgrounds. It is typically given an embed.FS.
func LoadBackgrounds(fsys fs.FS, pattern string) ([]image.Image, error) {
	names, err := fs.Glob(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("slidecaptcha: %w", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("slidecaptcha: no files match %q", pattern)
	}
	images := make([]image.Image, 0, len(names))
	for _, name := range names {
		f, err := fsys.Open(name)
		if err != nil {
			return nil, fmt.Errorf("slidecaptcha: %w", err)
		}
		img, _, err := image.Decode(f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("slidecaptcha: decode %s: %w", path.Base(name), err)
		}
		images = append(images, img)
	}
	return images, nil
}

// New returns a slider Verifier.
func New(opts ...Option) (*Verifier, error) {
	v := &Verifier{}
	for _, opt := range opts {
		opt(v)
	}
	return v, nil
}

// Challenge draws a new puzzle.
func (v *Verifier) Challenge(context.Context) (captcha.Challenge, error) {
	var bg *image.RGBA
	if len(v.backgrounds) > 0 {
		src := v.backgrounds[rand.IntN(len(v.backgrounds))]
		bg = image.NewRGBA(src.Rect)
		copy(bg.Pix, src.Pix)
	} else {
		bg = generate(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	}

	// The piece starts at the left edge; keep the cut-out clear of it.
	minX := pieceSize + 2*margin
	x := minX + rand.IntN(Width-pieceSize-margin-minX+1)
	y := margin + rand.IntN(Height-pieceSize-2*margin+1)

	piece := cutPiece(bg, x, y)
	punchHole(bg, x, y)

	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, bg, &jpeg.Options{Quality: 82}); err != nil {
		return captcha.Challenge{}, fmt.Errorf("slidecaptcha: encode background: %w", err)
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, piece); err != nil {
		return captcha.Challenge{}, fmt.Errorf("slidecaptcha: encode piece: %w", err)
	}
	payload, err := json.Marshal(Payload{
		Image:       "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpg.Bytes()),
		Thumb:       "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBuf.Bytes()),
		ThumbX:      0,
		ThumbY:      y,
		ThumbWidth:  pieceSize,
		ThumbHeight: pieceSize,
		Width:       Width,
		Height:      Height,
	})
	if err != nil {
		return captcha.Challenge{}, err
	}
	ans, err := json.Marshal(answer{X: x})
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

// cover scales img to fill Width×Height and crops the overflow evenly.
func cover(img image.Image) *image.RGBA {
	b := img.Bounds()
	scale := math.Max(float64(Width)/float64(b.Dx()), float64(Height)/float64(b.Dy()))
	w, h := int(math.Ceil(float64(b.Dx())*scale)), int(math.Ceil(float64(b.Dy())*scale))
	scaled := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(scaled, scaled.Rect, img, b, xdraw.Src, nil)
	out := image.NewRGBA(image.Rect(0, 0, Width, Height))
	offset := image.Pt((w-Width)/2, (h-Height)/2)
	xdraw.Draw(out, out.Rect, scaled, offset, xdraw.Src)
	return out
}

// palettes are sets of muted, neutral tones; one hue family is picked per
// image so the result stays calm.
var palettes = [][]color.RGBA{
	{{0x9f, 0xb3, 0xc8, 0xff}, {0x6e, 0x86, 0xa3, 0xff}, {0xc9, 0xd6, 0xe3, 0xff}, {0x4f, 0x63, 0x7d, 0xff}, {0xe4, 0xe9, 0xef, 0xff}},
	{{0xa8, 0xbf, 0xb5, 0xff}, {0x6f, 0x8f, 0x86, 0xff}, {0xd2, 0xdf, 0xd8, 0xff}, {0x4a, 0x66, 0x5f, 0xff}, {0xec, 0xf1, 0xee, 0xff}},
	{{0xc8, 0xbd, 0xae, 0xff}, {0x9a, 0x8b, 0x78, 0xff}, {0xe3, 0xdb, 0xd0, 0xff}, {0x6b, 0x5e, 0x50, 0xff}, {0xf1, 0xed, 0xe7, 0xff}},
	{{0xb8, 0xbb, 0xc0, 0xff}, {0x86, 0x8b, 0x93, 0xff}, {0xdc, 0xde, 0xe1, 0xff}, {0x5c, 0x61, 0x69, 0xff}, {0xf0, 0xf1, 0xf2, 0xff}},
}

// generate draws an abstract background: a soft vertical wash, overlapping
// translucent circles and bands, and fine grain so edges are not trivially
// detectable.
func generate(r *rand.Rand) *image.RGBA {
	pal := palettes[r.IntN(len(palettes))]
	img := image.NewRGBA(image.Rect(0, 0, Width, Height))
	top, bottom := pal[4], pal[2]
	for y := 0; y < Height; y++ {
		t := float64(y) / float64(Height-1)
		c := mix(top, bottom, t)
		for x := 0; x < Width; x++ {
			img.SetRGBA(x, y, c)
		}
	}

	type shape struct {
		cx, cy, rad float64
		c           color.RGBA
		alpha       float64
	}
	shapes := make([]shape, 7+r.IntN(5))
	for i := range shapes {
		shapes[i] = shape{
			cx:    r.Float64() * Width,
			cy:    r.Float64() * Height,
			rad:   24 + r.Float64()*90,
			c:     pal[r.IntN(4)],
			alpha: 0.35 + r.Float64()*0.45,
		}
	}
	// Diagonal bands.
	bands := 2 + r.IntN(3)
	type band struct {
		angle, offset, width float64
		c                    color.RGBA
		alpha                float64
	}
	bs := make([]band, bands)
	for i := range bs {
		bs[i] = band{
			angle:  (r.Float64()*0.8 - 0.4) + math.Pi/4*float64(1-2*r.IntN(2)),
			offset: r.Float64()*Width - Width/2,
			width:  10 + r.Float64()*36,
			c:      pal[r.IntN(4)],
			alpha:  0.18 + r.Float64()*0.25,
		}
	}

	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			c := img.RGBAAt(x, y)
			fx, fy := float64(x)+0.5, float64(y)+0.5
			for _, b := range bs {
				d := math.Abs((fx-Width/2)*math.Sin(b.angle)-(fy-Height/2)*math.Cos(b.angle)-b.offset) - b.width/2
				c = blend(c, b.c, b.alpha*smooth(d))
			}
			for _, s := range shapes {
				d := math.Hypot(fx-s.cx, fy-s.cy) - s.rad
				c = blend(c, s.c, s.alpha*smooth(d))
			}
			n := r.IntN(13) - 6
			c.R, c.G, c.B = clamp8(int(c.R)+n), clamp8(int(c.G)+n), clamp8(int(c.B)+n)
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func cutPiece(bg *image.RGBA, x, y int) *image.NRGBA {
	piece := image.NewNRGBA(image.Rect(0, 0, pieceSize, pieceSize))
	for py := 0; py < pieceSize; py++ {
		for px := 0; px < pieceSize; px++ {
			d := pieceDistance(float64(px)+0.5, float64(py)+0.5)
			cov := smooth(d)
			if cov <= 0 {
				continue
			}
			c := bg.RGBAAt(x+px, y+py)
			// A thin dark outline (no glow) so the piece reads on any background.
			if edge := cov - smooth(d+1.5); edge > 0 {
				c = blend(c, color.RGBA{0, 0, 0, 0xff}, 0.35*edge)
			}
			piece.SetNRGBA(px, py, color.NRGBA{c.R, c.G, c.B, uint8(cov * 255)})
		}
	}
	return piece
}

// pieceDistance is the signed distance (negative inside) from (px, py),
// relative to the piece's top-left corner, to the piece outline: a rounded
// square whose top edge carries a round tab.
func pieceDistance(px, py float64) float64 {
	const body = tabRadius
	const radius = 6.0
	qx := math.Abs(px-pieceSize/2) - (pieceSize/2 - radius)
	qy := math.Abs(py-(body+pieceSize)/2) - ((pieceSize-body)/2 - radius)
	rect := math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - radius
	tab := math.Hypot(px-pieceSize/2, py-body-1) - tabRadius
	return math.Min(rect, tab)
}

// punchHole darkens the cut-out and gives it a faint light rim.
func punchHole(bg *image.RGBA, x, y int) {
	for py := 0; py < pieceSize; py++ {
		for px := 0; px < pieceSize; px++ {
			fx, fy := float64(px)+0.5, float64(py)+0.5
			cov := smooth(pieceDistance(fx, fy))
			if cov <= 0 {
				continue
			}
			c := bg.RGBAAt(x+px, y+py)
			c = blend(c, color.RGBA{0, 0, 0, 0xff}, 0.38*cov)
			rim := cov - smooth(pieceDistance(fx, fy)+1.5)
			if rim > 0 {
				c = blend(c, color.RGBA{0xff, 0xff, 0xff, 0xff}, 0.3*rim)
			}
			bg.SetRGBA(x+px, y+py, c)
		}
	}
}

// smooth maps a signed distance (negative inside) to anti-aliased coverage.
func smooth(d float64) float64 {
	switch {
	case d <= -0.5:
		return 1
	case d >= 0.5:
		return 0
	default:
		return 0.5 - d
	}
}

func blend(dst, src color.RGBA, a float64) color.RGBA {
	if a <= 0 {
		return dst
	}
	if a > 1 {
		a = 1
	}
	return color.RGBA{
		R: uint8(float64(dst.R)*(1-a) + float64(src.R)*a),
		G: uint8(float64(dst.G)*(1-a) + float64(src.G)*a),
		B: uint8(float64(dst.B)*(1-a) + float64(src.B)*a),
		A: 0xff,
	}
}

func mix(a, b color.RGBA, t float64) color.RGBA { return blend(a, b, t) }

func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
