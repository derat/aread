package proc

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/derat/aread/common"
)

func runClean(w, h int, clr color.Color, maxw, maxh int) (image.Image, error) {
	td, err := ioutil.TempDir("", "image_cleaner_test.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(td)

	p := filepath.Join(td, "image.png")
	f, err := os.Create(p)
	if err != nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, clr)
		}
	}
	err = png.Encode(f, img)
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to encode to %v: %v", p, err)
	}

	ic := newImageCleaner(&common.Config{
		JPEGQuality:    90,
		Logger:         log.New(os.Stderr, "", log.LstdFlags),
		MaxImageBytes:  256 * 1024,
		MaxImageProcs:  2,
		MaxImageWidth:  maxw,
		MaxImageHeight: maxh,
	})
	if err := ic.clean(p); err != nil {
		return nil, err
	}

	if f, err = os.Open(p); err != nil {
		return nil, err
	}
	newImg, _, err := image.Decode(f)
	return newImg, err
}

func TestImageCleaner_square(t *testing.T) {
	img, err := runClean(400, 400, color.Black, 200, 200)
	if err != nil {
		t.Fatal("Clean failed: ", err)
	}
	if eb := image.Rect(0, 0, 200, 200); img.Bounds() != eb {
		t.Errorf("got bounds %v; want %v", img.Bounds(), eb)
	}
}

func TestImageCleaner_wide(t *testing.T) {
	img, err := runClean(400, 200, color.Black, 300, 50)
	if err != nil {
		t.Fatal("Clean failed: ", err)
	}
	if eb := image.Rect(0, 0, 100, 50); img.Bounds() != eb {
		t.Errorf("got bounds %v; want %v", img.Bounds(), eb)
	}
}

func TestImageCleaner_tall(t *testing.T) {
	img, err := runClean(200, 400, color.Black, 25, 350)
	if err != nil {
		t.Fatal("Clean failed: ", err)
	}
	if eb := image.Rect(0, 0, 25, 50); img.Bounds() != eb {
		t.Errorf("got bounds %v; want %v", img.Bounds(), eb)
	}
}

func TestImageCleaner_transparent(t *testing.T) {
	img, err := runClean(200, 200, color.Transparent, 100, 100)
	if err != nil {
		t.Fatal("Clean failed: ", err)
	}
	if eb := image.Rect(0, 0, 100, 100); img.Bounds() != eb {
		t.Errorf("got bounds %v; want %v", img.Bounds(), eb)
	}
	if !img.(*image.RGBA).Opaque() {
		t.Error("image was not made opaque")
	}
}
