package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"sync"

	"golang.org/x/image/draw"
)

type ImageCleaner struct {
	cfg            Config
	numImageProcs  int
	imageProcMutex sync.RWMutex
	imageProcCond  *sync.Cond
}

func newImageCleaner(cfg Config) *ImageCleaner {
	c := &ImageCleaner{cfg: cfg}
	c.imageProcCond = sync.NewCond(&c.imageProcMutex)
	return c
}

func (c *ImageCleaner) updateImage(src image.Image, imgFmt, filename string) error {
	sb := src.Bounds()
	needsScale := sb.Dx() > c.cfg.MaxImageWidth || sb.Dy() > c.cfg.MaxImageHeight
	needsOpaque := (src.ColorModel() == color.RGBAModel && !src.(*image.RGBA).Opaque()) ||
		(src.ColorModel() == color.NRGBAModel && !src.(*image.NRGBA).Opaque())
	if !needsScale && !needsOpaque {
		return nil
	}

	var dst *image.NRGBA
	var db image.Rectangle

	if needsScale {
		wr := float64(sb.Dx()) / float64(c.cfg.MaxImageWidth)
		hr := float64(sb.Dy()) / float64(c.cfg.MaxImageHeight)
		if wr > hr {
			db = image.Rect(0, 0, c.cfg.MaxImageWidth, int(float64(sb.Dy())/wr+0.5))
		} else {
			db = image.Rect(0, 0, int(float64(sb.Dx())/hr+0.5), c.cfg.MaxImageHeight)
		}

		c.cfg.Logger.Printf("Scaling %v from %vx%v to %vx%v\n",
			filename, sb.Dx(), sb.Dy(), db.Dx(), db.Dy())
		dst = image.NewNRGBA(db)
		draw.ApproxBiLinear.Scale(dst, db, src, sb, draw.Src, nil)
	}

	// 2nd-gen Kindles can't handle partially-transparent images. Shocking.
	if needsOpaque {
		if dst != nil {
			src = dst
			sb = dst.Bounds()
		} else {
			db = image.Rect(0, 0, sb.Dx(), sb.Dy())
			dst = image.NewNRGBA(db)
		}
		c.cfg.Logger.Printf("Making %v opaque\n", filename)
		for y := 0; y < sb.Dy(); y++ {
			for x := 0; x < sb.Dx(); x++ {
				cl := color.NRGBAModel.Convert(src.At(sb.Min.X+x, sb.Min.Y+y)).(color.NRGBA)
				if cl.A == 0 {
					cl.R = 255
					cl.G = 255
					cl.B = 255
					cl.A = 255
				}
				dst.SetNRGBA(db.Min.X+x, db.Min.Y+y, cl)
			}
		}
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	switch imgFmt {
	case "png":
		err = png.Encode(f, dst)
	case "jpeg":
		err = jpeg.Encode(f, dst, &jpeg.Options{Quality: c.cfg.JpegQuality})
	default:
		c.cfg.Logger.Fatalf("Unhandled image format %v for %v", imgFmt, filename)
	}
	return err
}

func (c *ImageCleaner) ProcessImage(filename string) error {
	c.imageProcCond.L.Lock()
	for c.numImageProcs >= c.cfg.MaxImageProcs {
		c.imageProcCond.Wait()
	}
	c.numImageProcs++
	c.imageProcCond.L.Unlock()

	defer func() {
		c.imageProcCond.L.Lock()
		c.numImageProcs--
		c.imageProcCond.L.Unlock()
		c.imageProcCond.Signal()
	}()

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	origInfo, err := f.Stat()
	if err != nil {
		return err
	}

	img, imgFmt, err := image.Decode(f)
	if err != nil {
		c.cfg.Logger.Printf("Unable to decode %v\n", filename)
	} else {
		if err = c.updateImage(img, imgFmt, filename); err != nil {
			return err
		}
	}

	newInfo, err := f.Stat()
	if err != nil {
		return err
	}
	if origInfo.Size() != newInfo.Size() {
		c.cfg.Logger.Printf("Resized %v from %v bytes to %v bytes\n", filename, origInfo.Size(), newInfo.Size())
	}
	if newInfo.Size() > c.cfg.MaxImageBytes {
		c.cfg.Logger.Printf("Deleting %v-byte file %v\n", newInfo.Size(), filename)
		if err = os.Remove(filename); err != nil {
			return err
		}
	}
	return nil
}
