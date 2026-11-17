package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/egoist/mygo/ui"

	"tungo/resources"
)

func drawMark(size int) image.Image {
	drawing := ui.MustParseSVG(resources.SVG)
	return ui.Render(func(c *ui.Context) {
		ui.Icon(c, drawing).Size(float32(size), float32(size))
	}, size, size, 1)
}

func appIcon(size int) []byte {
	pad := size * 14 / 100
	inner := size - 2*pad
	drawn := drawMark(inner)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	radius := float64(size) * 0.22

	const samples = 4
	const whole = samples * samples
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			cover := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					fx := float64(x) + (float64(sx)+0.5)/samples
					fy := float64(y) + (float64(sy)+0.5)/samples
					if inside(fx, fy, float64(size), radius) {
						cover++
					}
				}
			}
			if cover == 0 {
				continue
			}
			r, g, b := uint32(0xffff), uint32(0xffff), uint32(0xffff)
			if ix, iy := x-pad, y-pad; ix >= 0 && iy >= 0 && ix < inner && iy < inner {
				sr, sg, sb, sa := drawn.At(ix, iy).RGBA()
				keep := 0xffff - sa
				r, g, b = sr+keep, sg+keep, sb+keep
			}
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(min(r, 0xffff) >> 8),
				G: uint8(min(g, 0xffff) >> 8),
				B: uint8(min(b, 0xffff) >> 8),
				A: uint8(255 * cover / whole),
			})
		}
	}
	return encode(img)
}

func inside(x, y, size, radius float64) bool {
	dx := math.Max(radius-x, x-(size-radius))
	dy := math.Max(radius-y, y-(size-radius))
	if dx < 0 || dy < 0 {
		return x >= 0 && x <= size && y >= 0 && y <= size
	}
	return dx*dx+dy*dy <= radius*radius
}

func encode(img image.Image) []byte {
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}
