package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
)

func main() {
	for _, gray := range []bool{true, false} {
		for _, quality := range []int{1, 25, 50, 75, 90, 100} {
			for _, size := range []image.Point{{X: 9, Y: 7}, {X: 1, Y: 1}} {
				var src image.Image
				if gray {
					m := image.NewGray(image.Rect(0, 0, size.X, size.Y))
					for y := 0; y < size.Y; y++ {
						for x := 0; x < size.X; x++ {
							m.SetGray(x, y, color.Gray{Y: uint8((x*37 + y*53 + 11) % 256)})
						}
					}
					src = m
				} else {
					m := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
					for y := 0; y < size.Y; y++ {
						for x := 0; x < size.X; x++ {
							m.SetRGBA(x, y, color.RGBA{R: uint8((x*37 + y*13 + 7) % 256), G: uint8((x*11 + y*47 + 19) % 256), B: uint8((x*23 + y*29 + 31) % 256), A: 255})
						}
					}
					src = m
				}
				var buf bytes.Buffer
				if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: quality}); err != nil {
					panic(err)
				}
				decoded, err := jpeg.Decode(bytes.NewReader(buf.Bytes()))
				if err != nil {
					panic(err)
				}
				fmt.Printf("%s|", hex.EncodeToString(buf.Bytes()))
				for y := 0; y < size.Y; y++ {
					for x := 0; x < size.X; x++ {
						r, g, b, _ := decoded.At(x, y).RGBA()
						if x != 0 || y != 0 {
							fmt.Print(",")
						}
						fmt.Printf("%d,%d,%d", r>>8, g>>8, b>>8)
					}
				}
				fmt.Println()
			}
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "malformed" {
		fmt.Println("not-a-jpeg")
	}
}
