package main

import (
	"code.google.com/p/graphics-go/graphics"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"sync"
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

func (c *ImageCleaner) updateImage(origImg image.Image, imgFmt, filename string) error {
	origWidth := origImg.Bounds().Max.X - origImg.Bounds().Min.X
	origHeight := origImg.Bounds().Max.Y - origImg.Bounds().Min.Y
	needsScale := origWidth > c.cfg.MaxImageWidth || origHeight > c.cfg.MaxImageHeight
	needsOpaque := (origImg.ColorModel() == color.RGBAModel && !origImg.(*image.RGBA).Opaque()) ||
		(origImg.ColorModel() == color.NRGBAModel && !origImg.(*image.NRGBA).Opaque())
	if !needsScale && !needsOpaque {
		return nil
	}

	var newImg *image.NRGBA

	if needsScale {
		widthRatio := float64(origWidth) / float64(c.cfg.MaxImageWidth)
		heightRatio := float64(origHeight) / float64(c.cfg.MaxImageHeight)
		var newWidth, newHeight int
		if widthRatio > heightRatio {
			newWidth = c.cfg.MaxImageWidth
			newHeight = int(float64(origHeight)/widthRatio + 0.5)
		} else {
			newWidth = int(float64(origWidth)/heightRatio + 0.5)
			newHeight = c.cfg.MaxImageHeight
		}

		c.cfg.Logger.Printf("Scaling %v from %vx%v to %vx%v\n", filename, origWidth, origHeight, newWidth, newHeight)
		newImg = image.NewNRGBA(image.Rectangle{image.Point{0, 0}, image.Point{newWidth, newHeight}})
		if err := graphics.Scale(newImg, origImg); err != nil {
			return err
		}
	}

	// 2nd-gen Kindles can't handle partially-transparent images. Shocking.
	if needsOpaque {
		var srcImg image.Image
		if newImg != nil {
			srcImg = newImg
		} else {
			srcImg = origImg
			newImg = image.NewNRGBA(image.Rectangle{image.Point{0, 0}, image.Point{origWidth, origHeight}})
		}
		c.cfg.Logger.Printf("Making %v opaque\n", filename)
		for y := newImg.Bounds().Min.Y; y < newImg.Bounds().Max.Y; y++ {
			for x := newImg.Bounds().Min.X; x < newImg.Bounds().Max.X; x++ {
				cl := color.NRGBAModel.Convert(srcImg.At(x, y)).(color.NRGBA)
				if cl.A == 0 {
					cl.R = 255
					cl.G = 255
					cl.B = 255
					cl.A = 255
				}
				newImg.SetNRGBA(x, y, cl)
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
		err = png.Encode(f, newImg)
	case "jpeg":
		err = jpeg.Encode(f, newImg, &jpeg.Options{Quality: c.cfg.JpegQuality})
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
