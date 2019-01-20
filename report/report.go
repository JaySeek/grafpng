/*
   Copyright 2016 Vastech SA (PTY) LTD

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package report

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/negbie/reporter/grafana"
	"github.com/pborman/uuid"
)

// Report groups functions related to genrating the report.
type Report interface {
	Generate() (f io.ReadCloser, err error)
	Title() string
	Clean()
}

type report struct {
	gClient   grafana.Client
	time      grafana.TimeRange
	dashName  string
	tmpDir    string
	dashTitle string
}

// imageData struct fold holding each input image and related data
type imageData struct {
	img    image.Image
	width  int
	height int
	path   string
}

const (
	imgDir = "images"
)

// New creates a new Report.
func New(g grafana.Client, dashName string, time grafana.TimeRange) Report {
	return new(g, dashName, time)
}

func new(g grafana.Client, dashName string, time grafana.TimeRange) *report {
	tmpDir := filepath.Join("tmp", uuid.New())
	return &report{g, time, dashName, tmpDir, ""}
}

// Generate returns the png file. After reading this file it should be Closed()
// After closing the file, call report.Clean() to delete the file as well the temporary build files
func (rep *report) Generate() (f io.ReadCloser, err error) {
	dash, err := rep.gClient.GetDashboard(rep.dashName)
	if err != nil {
		err = fmt.Errorf("error fetching dashboard %v: %v", rep.dashName, err)
		return
	}
	rep.dashTitle = dash.Title

	fn, err := rep.renderPNGsParallel(dash)
	if err != nil {
		err = fmt.Errorf("error rendering PNGs in parralel for dash %+v: %v", dash, err)
		return
	}

	return os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0755)
}

// Title returns the dashboard title parsed from the dashboard definition
func (rep *report) Title() string {
	//lazy fetch if Title() is called before Generate()
	if rep.dashTitle == "" {
		dash, err := rep.gClient.GetDashboard(rep.dashName)
		if err != nil {
			return ""
		}
		rep.dashTitle = dash.Title
	}
	return rep.dashTitle
}

// Clean deletes the temporary directory used during report generation
func (rep *report) Clean() {
	err := os.RemoveAll(rep.tmpDir)
	if err != nil {
		log.Println("Error cleaning up tmp dir:", err)
	}
}

func (rep *report) imgDirPath() string {
	return filepath.Join(rep.tmpDir, imgDir)
}

func (rep *report) renderPNGsParallel(dash grafana.Dashboard) (string, error) {
	//buffer all panels on a channel
	panels := make(chan grafana.Panel, len(dash.Panels))
	for _, p := range dash.Panels {
		panels <- p
	}
	close(panels)
	images := make([]*imageData, len(dash.Panels))

	//fetch images in parrallel form Grafana sever.
	//limit concurrency using a worker pool to avoid overwhelming grafana
	//for dashboards with many panels.
	var wg sync.WaitGroup
	workers := runtime.NumCPU()
	wg.Add(workers)
	var j uint64
	errs := make(chan error, len(dash.Panels)) //routines can return errors on a channel
	for i := 0; i < workers; i++ {
		go func(panels <-chan grafana.Panel, errs chan<- error) {
			defer wg.Done()
			for p := range panels {
				filename, err := rep.renderPNG(p)
				if err != nil {
					log.Printf("Error creating image for panel: %v", err)
					errs <- err
				}
				fimg, err := os.Open(filename)
				if err != nil {
					log.Fatal("Unable to open file", filename)
				}
				defer fimg.Close()
				// Decode the file to get the image data
				img, _, err := image.Decode(fimg)
				if err != nil {
					log.Fatal("Unable to decode ", filename)
				}
				// Fill image data object
				imd, err := getImageData(&img, filename)
				if err != nil {
					log.Fatal(err)
				}
				// Append to imadeData array
				images[atomic.LoadUint64(&j)] = &imd
				atomic.AddUint64(&j, 1)
			}
		}(panels, errs)

	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return "", err
		}
	}

	return processImages(images, dash.Title)
}

func (rep *report) renderPNG(p grafana.Panel) (string, error) {
	body, err := rep.gClient.GetPanelPng(p, rep.dashName, rep.time)
	if err != nil {
		return "", fmt.Errorf("error getting panel %+v: %v", p, err)
	}
	defer body.Close()

	err = os.MkdirAll(rep.imgDirPath(), 0777)
	if err != nil {
		return "", fmt.Errorf("error creating img directory:%v", err)
	}
	fmt.Println(rep.imgDirPath())
	imgFileName := fmt.Sprintf("image%d.png", p.Id)
	file, err := os.Create(filepath.Join(rep.imgDirPath(), imgFileName))
	if err != nil {
		return "", fmt.Errorf("error creating image file:%v", err)
	}
	defer file.Close()
	fmt.Println(file.Name())

	_, err = io.Copy(file, body)
	if err != nil {
		return "", fmt.Errorf("error copying body to file:%v", err)
	}

	return file.Name(), nil
}

// getImageData function to populate a imageData object with input image details
// Takes the image, and filename as arguments
// Returns the filled imageData object and an error if any
func getImageData(img *image.Image, filename string) (imageData, error) {
	imd := &imageData{}
	imd.img = *img
	imd.path = filename
	h, w, err := getDim(imd)
	imd.height, imd.width = h, w
	if err != nil {
		return *imd, err
	}

	return *imd, nil

}

// getDim function to get the dimensions of an input image
// Takes imageData as argument
// Return height, width and error if any
func getDim(imd *imageData) (int, int, error) {
	f, err := os.Open(imd.path)
	if err != nil {
		return -1, -1, err
	}
	defer f.Close()
	// Decode config of image to get height and width
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return -1, -1, err
	}
	return config.Height, config.Width, nil
}

// getTotalDim function to get the total height and width
// i.e, sum of widths and heights of all input images
// Takes the array of imageData as argument
// Returns total height, width and error if any
func getTotalDim(images []*imageData) (int, int, error) {
	height, width := 0, 0
	// Loop through images and add the height and width
	for _, imd := range images {
		height = height + imd.height
		width = width + imd.width
	}

	if height == 0 && width == 0 {
		return height, width, errors.New("total Height and Width cannot be 0")
	}

	return height, width, nil
}

// getMaxDim function to get the maximum width and height from
// all the input images. Takes imageData array as argument
// Returns max height, width and error if any
func getMaxDim(images []*imageData) (int, int, error) {
	maxh, maxw := 0, 0
	// Loop through images to find the largest height and width
	for _, imd := range images {
		if imd.height > maxh {
			maxh = imd.height
		}
		if imd.width > maxw {
			maxw = imd.width
		}
	}
	return maxh, maxw, nil
}

// processImages function to loop through all images in the imageData array
// and calculate the total height, width and max height, width.
// Finally calls makeImage to create the image
// Takes the array of imageData, format and side as arguments
func processImages(images []*imageData, outfile string) (out string, err error) {
	th, tw, err := getTotalDim(images)
	if err != nil {
		return "", err
	}
	maxh, maxw, err := getMaxDim(images)
	if err != nil {
		return "", err
	}
	// Create the output image
	out, err = makeImage(th, tw, maxh, maxw, images, outfile)
	if err != nil {
		return "", err
	}
	return out, nil
}

// makeImage function to create the combined image from all the input images
// Takes total height, width, max height, width, input images, format to
// encode. Returns error if any
func makeImage(th, tw, maxh, maxw int, images []*imageData, outfile string) (string, error) {
	var img *image.RGBA
	posx, posy := 0, 0

	img = image.NewRGBA(image.Rect(0, 0, maxw, th))
	for _, imd := range images {
		r := image.Rect(posx, posy, posx+imd.width, posy+imd.height)
		draw.Draw(img, r, imd.img, image.Point{0, 0}, draw.Over)
		posy = posy + imd.height
	}

	file := outfile + ".png"
	out, err := os.Create(file)
	if err != nil {
		return "", err
	}
	defer out.Close()

	err = png.Encode(out, img)
	if err != nil {
		return "", err
	}

	return file, nil
}
