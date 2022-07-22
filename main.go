package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"sync"
	"time"
)

var wg sync.WaitGroup

var inputFile string
var outputFile string
var outputFormat string
var threshold uint

func main() {
	flag.StringVar(&inputFile, "i", "", "Input file. Supported types: png, jpg, gif")
	flag.StringVar(&outputFile, "o", "output", "Output file name")
	flag.StringVar(&outputFormat, "f", "", "Format for the output file (png, jpg, gif). Defaults to using the same as input file")
	flag.UintVar(&threshold, "t", 10, "Quality threshold")

	// Customize usage text
	flag.Usage = func() {
		fmt.Println("Usage for QuadTreeImageCompression 1.0.0: ")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Validate input file parameter
	if inputFile == "" {
		fmt.Println("Missing input file parameter!")
		flag.Usage()
		os.Exit(1)
	}

	// Validate output format parameter
	if outputFormat != "png" && outputFormat != "jpg" && outputFormat != "gif" && outputFormat != "" {
		fmt.Println("Invalid output file format!", outputFormat)
		flag.Usage()
		os.Exit(1)
	}

	imageFile, err := os.Open(inputFile)
	if err != nil {
		log.Fatal("Error opening input file!\n", err)
	}
	defer imageFile.Close()

	input, format, err := image.Decode(imageFile)
	if err != nil {
		log.Fatal("Error decoding input image!\n", err)
	}

	log.Println("Image type:", format)
	log.Println("Resolution:", input.Bounds().Size())
	log.Println("Threshold:", threshold)

	// Converts the input image into a non-alpha-premultiplied color format
	rgbImg := image.NewNRGBA(input.Bounds())
	draw.Draw(rgbImg, input.Bounds(), input, image.Point{}, draw.Src)

	start := time.Now()

	// Start of the compression
	wg.Add(1)
	processStep(rgbImg, input.Bounds())
	wg.Wait()

	log.Println("Compression done! Took:", time.Since(start))

	if outputFormat == "" {
		outputFormat = format
	}

	outFile := fmt.Sprintf("%s.%s", outputFile, outputFormat)
	log.Println("Saving into", outFile, "...")
	output, err := os.Create(outFile)
	if err != nil {
		log.Fatal("Error creating output file!\n", err)
	}
	defer output.Close()

	if outputFormat == "png" {
		err = png.Encode(output, rgbImg)
	} else if outputFormat == "gif" {
		err = gif.Encode(output, rgbImg, &gif.Options{
			NumColors: 256,
			Quantizer: nil,
			Drawer:    nil,
		})
	} else {
		err = jpeg.Encode(output, rgbImg, &jpeg.Options{
			Quality: jpeg.DefaultQuality,
		})
	}
	if err != nil {
		log.Fatal("Error encoding output file!\n", err)
	}
}

func processStep(output *image.NRGBA, region image.Rectangle) {
	width, height := region.Dx(), region.Dy()

	regionImg := output.SubImage(region).(*image.NRGBA)

	// Calculate average color and error
	avg := colorAverage(regionImg)
	errorVal := regionError(regionImg, avg)

	// Split image in four regions
	if errorVal > int(threshold) && width >= 2 && height >= 2 {
		// Top left
		wg.Add(1)
		go processStep(output,
			image.Rect(0, 0, width/2, height/2).Add(region.Min),
		)

		// Top right
		wg.Add(1)
		go processStep(output,
			image.Rect(width/2, 0, width, height/2).Add(region.Min),
		)

		// Bottom left
		wg.Add(1)
		go processStep(output,
			image.Rect(0, height/2, width/2, height).Add(region.Min),
		)

		// Bottom right
		wg.Add(1)
		go processStep(output,
			image.Rect(width/2, height/2, width, height).Add(region.Min),
		)
	} else {
		// Fill region with the average color
		draw.Draw(output, region, &image.Uniform{C: avg}, image.Point{}, draw.Src)
	}

	wg.Done()
}

func colorAverage(image *image.NRGBA) color.NRGBA {
	size := image.Bounds()
	minX, minY := size.Min.X, size.Min.Y
	maxX, maxY := size.Max.X, size.Max.Y

	n := size.Dx() * size.Dy()
	sum := make([]uint32, 3)

	for y := minY; y < maxY; y++ {
		offset := image.PixOffset(minX, y)
		for x := minX; x < maxX; x++ {
			sum[0] += uint32(image.Pix[offset+0])
			sum[1] += uint32(image.Pix[offset+1])
			sum[2] += uint32(image.Pix[offset+2])
			offset += 4
		}
	}

	avgR := uint8(sum[0] / uint32(n))
	avgG := uint8(sum[1] / uint32(n))
	avgB := uint8(sum[2] / uint32(n))
	return color.NRGBA{R: avgR, G: avgG, B: avgB, A: 255}
}

func regionError(image *image.NRGBA, averageColor color.NRGBA) int {
	size := image.Bounds()
	minX, minY := size.Min.X, size.Min.Y
	maxX, maxY := size.Max.X, size.Max.Y

	n := size.Dx() * size.Dy()
	sum := make([]int, 3)

	for y := minY; y < maxY; y++ {
		offset := image.PixOffset(minX, y)
		for x := minX; x < maxX; x++ {
			sum[0] += absDiff(int(image.Pix[offset+0]), int(averageColor.R))
			sum[1] += absDiff(int(image.Pix[offset+1]), int(averageColor.G))
			sum[2] += absDiff(int(image.Pix[offset+2]), int(averageColor.B))
			offset += 4
		}
	}

	return (sum[0] + sum[1] + sum[2]) / (n * 3)
}

func absDiff(a int, b int) int {
	result := a - b
	if result < 0 {
		return -result
	}
	return result
}
