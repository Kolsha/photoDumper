package vk

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Gasoid/photoDumper/sources"
	"github.com/SevereCloud/vksdk/v2/object"
)

// PhotoItem is a struct that contains a directory, a URL, a creation time, an album name, and a
// longitude and latitude.
type PhotoItem struct {
	url       []string
	created   time.Time
	albumName string
	longitude,
	latitude float64
	extension   string
	description string
	sourceUrl   string
}

func (f *PhotoItem) Url() []string {
	return f.url
}
func (f *PhotoItem) SourceUrl() string {
	return f.sourceUrl
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
	if f.description == "" {
		f.description = fmt.Sprintf("Album name: %s", f.albumName)
	}

	exif := &exifInfo{
		description: f.description,
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

func area(b object.PhotosPhotoSizes) float64 {
	return b.Height * b.Width
}

func toPhotoItem(photo object.PhotosPhoto) PhotoItem {
	sort.Slice(photo.Sizes, func(i, j int) bool {
		return area(photo.Sizes[i]) > area(photo.Sizes[j])
	})

	urls := make([]string, 0, len(photo.Sizes))
	for _, s := range photo.Sizes {
		urls = append(urls, s.URL)
	}
	created := time.Unix(int64(photo.Date), 0)
	description := photo.Text + " " + photo.Description + " " + photo.Title
	description = strings.Trim(description, " \t\n\r")
	return PhotoItem{
		url:         urls,
		created:     created,
		latitude:    photo.Lat,
		longitude:   photo.Long,
		extension:   "jpg",
		description: description,
		sourceUrl:   fmt.Sprintf("https://vk.com/%s", photo.ToAttachment()),
	}
}
