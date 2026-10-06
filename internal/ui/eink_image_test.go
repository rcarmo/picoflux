package ui

import (
	"image"
	"testing"
)

func BenchmarkAdaptImageForEInk(b *testing.B) {
	src := image.NewRGBA(image.Rect(0, 0, 900, 700))
	for i := range src.Pix {
		src.Pix[i] = byte(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adaptImageForEInk(src)
	}
}

// Compare the direct pixel path with the established interface-based luma
// formula, including non-zero image origins and premultiplied alpha.
func TestEInkDirectPixelConversion(t *testing.T) {
	src := image.NewRGBA(image.Rect(7, 9, 37, 29))
	for i := range src.Pix {
		src.Pix[i] = byte(i)
	}
	got := adaptImageForEInk(src)
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			r, g, blue, _ := src.At(x, y).RGBA()
			want := int((299*(r>>8) + 587*(g>>8) + 114*(blue>>8)) / 1000)
			want = 128 + (want-128)*145/100
			want = max(0, min(255, want))
			value, _, _, _ := got.At(x, y).RGBA()
			if int(value>>8) != want {
				t.Fatalf("pixel (%d,%d): got %d want %d", x, y, value>>8, want)
			}
		}
	}
}

func TestEInkSubimageStride(t *testing.T) {
	parent := image.NewRGBA(image.Rect(0, 0, 80, 60))
	for i := range parent.Pix {
		parent.Pix[i] = byte(i * 7)
	}
	src := parent.SubImage(image.Rect(11, 13, 39, 31)).(*image.RGBA)
	got := adaptImageForEInk(src)
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			r, g, blue, _ := src.At(x, y).RGBA()
			want := int((299*(r>>8) + 587*(g>>8) + 114*(blue>>8)) / 1000)
			want = max(0, min(255, 128+(want-128)*145/100))
			value, _, _, _ := got.At(x, y).RGBA()
			if int(value>>8) != want {
				t.Fatalf("pixel (%d,%d): got %d want %d", x, y, value>>8, want)
			}
		}
	}
}
