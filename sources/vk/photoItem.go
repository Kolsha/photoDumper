package vk

import (
	"fmt"
	"strings"
	"time"

	"github.com/Gasoid/photoDumper/sources"
)

// PhotoItem is a struct that contains a directory, a URL, a creation time, an album name, and a
// longitude and latitude.
type PhotoItem struct {
	url       []string
	created   time.Time
	albumName string
	longitude,
	latitude float64
	extension string
}

func (f *PhotoItem) Url() []string {
	return f.url
}

func (f *PhotoItem) AlbumName() string {
	return f.albumName
}

func (f *PhotoItem) FileName() string {
	if f.extension == "" {
		return ""
	}
	filename := f.created.Format(time.RFC3339)
	filename = strings.ReplaceAll(filename, ":", "-")
	return fmt.Sprintf("%s.%s", filename, f.extension)
}

// It's setting EXIF data for the downloaded file.
func (f *PhotoItem) ExifInfo() (sources.ExifInfo, error) {
	exif := &exifInfo{
		description: fmt.Sprintf("Dumped by photoDumper. Source is vk. Album name: %s", f.albumName),
		created:     f.created,
		gps:         []float64{f.latitude, f.longitude},
	}
	return exif, nil
}

type exifInfo struct {
	description string
	created     time.Time
	gps         []float64
}

func (e *exifInfo) Description() string {
	return e.description
}

func (e *exifInfo) Created() time.Time {
	return e.created
}

func (e *exifInfo) GPS() []float64 {
	return e.gps
}
