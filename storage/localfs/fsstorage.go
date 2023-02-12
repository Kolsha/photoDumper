package localfs

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Gasoid/photoDumper/sources"
	exif "github.com/Gasoid/simpleGoExif"
	exiftool "github.com/barasher/go-exiftool"
	exifAnother "github.com/dsoprea/go-exif/v2"
)

type SimpleStorage struct {
}

// It's a method of Social struct. It's checking if the path is absolute or relative.
func (s *SimpleStorage) dirPath(dir string) (string, error) {
	if len(dir) < 1 {
		return "", fmt.Errorf("len of dir is less 1")
	}
	if dir[:1] == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, filepath.FromSlash(dir[1:]))
	}
	return dir, nil
}

func (s *SimpleStorage) Prepare(dir string) (string, error) {
	dir, err := s.dirPath(dir)
	if err != nil {
		log.Println("prepareDir", err)
		return "", err
	}
	err = os.MkdirAll(dir, 0750)
	return dir, err
}

// It takes a URL, parses it, and returns the base name of the path
func (s *SimpleStorage) FilePath(dir, filename string) string {
	return filepath.Join(dir, filename)
}

func filename(path string) (string, error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	name := filepath.Base(u.Path)
	if filepath.Ext(name) == "" {
		return "", errors.New("no ext")
	}
	return name, nil
}

func (s *SimpleStorage) CreateAlbumDir(rootDir, albumName string) (string, error) {
	albumDir := filepath.Join(rootDir, albumName)
	err := os.MkdirAll(albumDir, 0750)
	if err != nil {
		return "", fmt.Errorf("createAlbumDir: %w", err)
	}
	return albumDir, nil
}

var backoffSchedule = []time.Duration{
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
	32 * time.Second,
}

var (
	once       sync.Once
	netClient  *http.Client
	seenURLMap sync.Map
)

func seenUrl(url string) bool {
	backOff := 0
	for {
		actual, loaded := seenURLMap.LoadOrStore(url, nil)
		if !loaded {
			return false
		}
		if actual == nil {
			time.Sleep(backoffSchedule[backOff])
			backOff++
			if backOff >= len(backoffSchedule) {
				backOff = 0
			}
			continue
		}
		return true
	}
}

func removeFromSeen(url string) {
	seenURLMap.CompareAndDelete(url, nil)
}
func markAsSeen(url string) {
	seenURLMap.CompareAndSwap(url, nil, true)
}

func newNetClient() *http.Client {
	once.Do(func() {

		t := http.DefaultTransport.(*http.Transport).Clone()
		t.MaxIdleConns = 100
		t.MaxConnsPerHost = 100
		t.MaxIdleConnsPerHost = 100

		netClient = &http.Client{
			Transport: t,
		}
	})

	return netClient
}

func getURLDataWithRetries(url string) (io.ReadCloser, error) {
	var err error
	var resp *http.Response

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for _, backoff := range backoffSchedule {
		resp, err = newNetClient().Do(req)

		if err == nil {
			status := resp.StatusCode
			body := resp.Body
			if status == http.StatusOK {
				return body, nil
			}
			defer body.Close()
			io.Copy(io.Discard, body)

			err = fmt.Errorf("%q is unavailable. code is %d", url, resp.StatusCode)
			if status == http.StatusNotFound {
				return nil, err
			}

		}

		time.Sleep(backoff)
	}

	// All retries failed
	return nil, err
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// It downloads the file from the url, creates a file with the name of the file, and writes the body of
// the response to the file
func (s *SimpleStorage) DownloadPhoto(url, dir, fn string) (string, error) {
	var err error
	defer func() {
		if err != nil {
			removeFromSeen(url)
		} else {
			markAsSeen(url)
		}
	}()

	if seenUrl(url) {
		log.Println("files is seen", url)
		return "", nil
	}

	body, err := getURLDataWithRetries(url)
	if err != nil {
		log.Println(err)
		return "", err
	}
	defer body.Close()

	if fn == "" {
		fn = randSeq(10) + ".jpg"
	}
	urlName, _ := filename(url)
	fn = fmt.Sprintf("%s_%s", urlName, fn)
	filepath := s.FilePath(dir, fn)

	out, err := os.Create(filepath)
	if err != nil {
		io.Copy(io.Discard, body)
		log.Println(err)
		return "", err
	}
	defer out.Close()

	// Write the body to file

	_, err = io.Copy(out, body)
	if err != nil {
		return "", err
	}

	return filepath, nil
}

func tryExifTool(filepath string, photoExif sources.ExifInfo) error {
	et, err := exiftool.NewExiftool()
	if err != nil {
		return err
	}
	defer et.Close()

	originals := et.ExtractMetadata(filepath)
	if len(originals) < 1 || originals[0].Fields == nil || originals[0].Err != nil {
		return fmt.Errorf("unknown metadata")
	}
	originals[0].SetString("Title", photoExif.Description())
	originals[0].SetString("DateTime", exifAnother.ExifFullTimestampString(photoExif.Created()))
	et.WriteMetadata(originals)
	return nil
}

// It's setting EXIF data for the downloaded file.
func (s *SimpleStorage) SetExif(filepath string, photoExif sources.ExifInfo) error {
	image, err := exif.Open(filepath)
	if err != nil {
		// log.Println("exif.Open", err)
		err = tryExifTool(filepath, photoExif)
		if err != nil {
			log.Println("tryExifTool", err)
		}
		return err
	}
	defer image.Close()
	if photoExif == nil {
		return errors.New("exif is empty")
	}
	err = image.SetDescription(photoExif.Description())
	if err != nil {
		return err
	}
	err = image.SetTime(photoExif.Created())
	if err != nil {
		return err
	}
	gps := photoExif.GPS()
	if gps == nil {
		return errors.New("gps is empty")
	}
	err = image.SetGPS(gps[0], gps[1])
	if err != nil {
		return err
	}

	return nil
}

func New() sources.Storage {
	return &SimpleStorage{}
}

type service struct{}

func (s *service) Kind() sources.Kind {
	return sources.KindStorage
}

func (s *service) Key() string {
	return "fs"
}

func (s *service) Constructor() func() sources.Storage {
	return New
}

func NewService() sources.ServiceStorage {
	return &service{}
}
