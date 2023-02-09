package sources

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"
)

type Kind int

const (
	KindSource Kind = iota
	KindStorage
)

var (
	registeredSources  = map[string]func(creds string) Source{}
	registeredStorages = map[string]func() Storage{}
	photoCh            chan payload
	maxConcurrentFiles = 20
)

type StorageError struct {
	text string
	err  error
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("Source error: %s", e.text)
}

func (e *StorageError) Unwrap() error {
	return e.err
}

type SourceError struct {
	text string
	err  error
}

func (e *SourceError) Error() string {
	return fmt.Sprintf("Source error: %s", e.text)
}

func (e *SourceError) Unwrap() error {
	return e.err
}

type AccessError struct {
	Text string
	Err  error
}

func (e *AccessError) Error() string {
	return fmt.Sprintf("Auth error: %s", e.Text)
}

func (e *AccessError) Unwrap() error {
	return e.Err
}

type ItemFetcher interface {
	Next() bool
	Item() Photo
}

type Source interface {
	AllAlbums() ([]map[string]string, error)
	AlbumPhotos(albumdID string) (ItemFetcher, error)

	ConversationPhotos(peerId, title string) (ItemFetcher, error)
	AllConversations() ([]map[string]string, error)
}

type ExifInfo interface {
	Description() string
	Created() time.Time
	GPS() []float64
}

type Photo interface {
	Url() []string
	AlbumName() string
	ExifInfo() (ExifInfo, error)
	FileName() string
	SourceUrl() string
}

type payload struct {
	photo   Photo
	rootDir string
}

type Storage interface {
	Prepare(dir string) (string, error)
	CreateAlbumDir(rootDir, dir string) (string, error)
	DownloadPhoto(photoUrl, dir, filename string) (string, error)
	SetExif(filepath string, info ExifInfo) error
}

type Social struct {
	source  Source
	storage Storage
}

// Albums returns albums
func (s *Social) Albums() ([]map[string]string, error) {
	albums, err := s.source.AllAlbums()
	if err != nil {
		return nil, err
	}
	return albums, nil
}

func (s *Social) DownloadAllAlbums(dir string) (string, error) {
	dir, err := s.storage.Prepare(dir)
	if err != nil {
		log.Println("DownloadAllAlbums(dir string)", err)
		return "", &StorageError{text: "dir can't be created", err: err}
	}

	albums, err := s.source.AllAlbums()
	if err != nil {
		return "", err
	}
	for _, album := range albums {
		go func(albumID string) {
			_, err := s.DownloadAlbum(albumID, dir)
			if err != nil {
				log.Println(err, "DownloadAllAlbums failed")
			}
		}(album["id"])
	}

	return dir, nil
}

// DownloadAlbum runs copying process to a particular directory
func (s *Social) DownloadAlbum(albumID, dir string) (string, error) {
	dir, err := s.storage.Prepare(dir)
	if err != nil {
		log.Println("DownloadAlbum(albumID, dir string)", err)
		return "", &StorageError{text: "dir can't be created", err: err}
	}
	cur, err := s.source.AlbumPhotos(albumID)
	if err != nil {
		return "", &SourceError{text: "can't receive photos", err: err}
	}
	go func() {
		for cur.Next() {
			photoCh <- payload{photo: cur.Item(), rootDir: dir}
		}
	}()
	return dir, nil
}

// DownloadConversationPhotos
func (s *Social) DownloadConversationPhotos(peerId, title, dir string) (string, error) {
	dir, err := s.storage.Prepare(dir)
	if err != nil {
		log.Println("DownloadConversationPhotos", err)
		return "", &StorageError{text: "dir can't be created", err: err}
	}

	cur, err := s.source.ConversationPhotos(peerId, title)
	if err != nil {
		return "", &SourceError{text: "DownloadConversationPhotos: can't receive photos", err: err}
	}
	// go func() {
	for cur.Next() {
		photoCh <- payload{photo: cur.Item(), rootDir: dir}
	}
	// }()
	return dir, nil
}

func (s *Social) DownloadAllConversations(dir string) (string, error) {
	// return s.DownloadConversationPhotos("306711398", "", dir)
	dir, err := s.storage.Prepare(dir)
	if err != nil {
		log.Println("DownloadAllConversations", err)
		return "", &StorageError{text: "dir can't be created", err: err}
	}

	conversations, err := s.source.AllConversations()
	log.Println(conversations)
	if err != nil {
		return "", err
	}

	sem := semaphore.NewWeighted(5)
	ctx := context.TODO()

	for _, item := range conversations {
		go func(id, title string) {

			sem.Acquire(ctx, 1)
			defer sem.Release(1)

			_, err := s.DownloadConversationPhotos(id, title, dir)
			if err != nil {
				log.Println(err, "DownloadAllConversations failed")
			}
		}(item["id"], item["title"])
	}

	return dir, nil
}

func saveFailedPhotoInfo(f payload) {
	sourceUrl := f.photo.SourceUrl()
	attach := strings.Split(sourceUrl, "/")
	fn := attach[len(attach)-1]
	filepath := filepath.Join(f.rootDir, fn+".txt")
	out, err := os.Create(filepath)
	if err != nil {
		log.Println(err)
		return
	}
	defer out.Close()
	out.WriteString(sourceUrl)
}

func (s *Social) savePhotos(photoCh chan payload) {
	log.Println("savePhotos: started")
	for file := range photoCh {
		// log.Println("Got file", file)
		f := file
		/*go*/ func() {
			dir, err := s.storage.CreateAlbumDir(f.rootDir, f.photo.AlbumName())
			if err != nil {
				log.Println(err)
				return
			}
			var filepath string
			for _, url := range f.photo.Url() {
				filepath, err = s.storage.DownloadPhoto(url, dir, f.photo.FileName())
				if err != nil {
					log.Println(err)
				} else {
					break
				}
			}

			if err != nil {
				log.Printf("FILE FAILED DOWNLOAD: %+v\n", err)
				saveFailedPhotoInfo(f)
				return
			}
			exif, err := f.photo.ExifInfo()
			if err != nil {
				log.Println(err)
				return
			}
			if exif == nil {
				return
			}
			s.storage.SetExif(filepath, exif)
		}()
	}
	log.Println("channel closed")
}

// New creates a new instance of Social, you have to provide proper options
func New(sourceName, creds string) (*Social, error) {
	source, err := ProvideSource(sourceName, creds)
	if err != nil {
		return nil, err
	}
	storage, err := ProvideStorage()
	if err != nil {
		return nil, err
	}
	s := &Social{
		storage: storage,
		source:  source,
	}
	if photoCh == nil {
		photoCh = make(chan payload, maxConcurrentFiles)
		for i := 0; i < maxConcurrentFiles; i++ {
			go s.savePhotos(photoCh)
		}
	}
	return s, nil
}

type Service interface {
	Key() string
}

type ServiceSource interface {
	Service
	Constructor() func(creds string) Source
}

type ServiceStorage interface {
	Service
	Constructor() func() Storage
}

func AddSource(s ServiceSource) {
	registeredSources[s.Key()] = s.Constructor()
}

func AddStorage(s ServiceStorage) {
	registeredStorages[s.Key()] = s.Constructor()
}

func ProvideSource(key string, creds string) (Source, error) {
	if newFunc, ok := registeredSources[key]; ok {
		return newFunc(creds), nil
	} else {
		return nil, &SourceError{text: "Source was not found"}
	}
}

func ProvideStorage() (Storage, error) {
	for _, v := range registeredStorages {
		return v(), nil
	}
	return nil, &StorageError{text: "no storages"}
}

func Sources() []string {
	listSources := make([]string, len(registeredSources))
	var i int
	for key := range registeredSources {
		listSources[i] = key
		i++
	}
	return listSources
}
