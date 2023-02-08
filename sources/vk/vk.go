package vk

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Gasoid/photoDumper/sources"
	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/object"
)

const (
	maxCount = 1000
)

type Vk struct {
	vkAPI *api.VK
}

// PhotoItem is a struct that contains a directory, a URL, a creation time, an album name, and a
// longitude and latitude.
type PhotoItem struct {
	url       []string
	created   time.Time
	albumName string
	longitude,
	latitude float64
}

func (f *PhotoItem) Url() []string {
	return f.url
}

func (f *PhotoItem) AlbumName() string {
	return f.albumName
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

// It creates a new Vk object, which is a wrapper around the vkAPI object
func New(creds string) sources.Source {
	return &Vk{vkAPI: api.NewVK(creds)}
}

// Getting albums from vk api
func (v *Vk) AllAlbums() ([]map[string]string, error) {
	resp, err := v.vkAPI.PhotosGetAlbums(api.Params{"need_covers": 1})
	if err != nil {
		return nil, makeError(err, "GetAlbums failed")
	}
	albums := make([]map[string]string, resp.Count)
	for i, album := range resp.Items {
		if album.ID < 0 {
			continue
		}
		created := time.Unix(int64(album.Created), 0)
		albums[i] = map[string]string{
			"thumb":   album.ThumbSrc,
			"title":   album.Title,
			"id":      fmt.Sprint(album.ID),
			"created": created.Format(time.RFC3339),
			"size":    fmt.Sprint(album.Size),
			// "count": album.,
		}
	}
	return albums, nil
}

func (v *Vk) AllConversations() ([]map[string]string, error) {
	const maxConversations = 200
	params := api.Params{
		"offset":   0,
		"count":    maxConversations,
		"extended": 1,
		// "v":      "5.80",
	}
	result := make([]map[string]string, 0, 1)

	var resp api.MessagesGetConversationsResponse
	var err error
	offset := 0
	for {
		resp, err = v.vkAPI.MessagesGetConversations(params)
		if err != nil {
			log.Println("AllConversations:", err)
			return nil, makeError(err, "AllConversations failed")
		}
		size := len(resp.Items)
		if size < 1 {
			break
		}
		offset += size
		params["offset"] = offset

		log.Printf("Getting conversation: %d got", size)

		titles := map[int]string{}
		for _, item := range resp.Profiles {
			titles[item.ID] = fmt.Sprintf("%s_%s_%d", item.FirstName, item.LastName, item.ID)
		}
		for _, item := range resp.Groups {
			titles[-item.ID] = fmt.Sprintf("%s_-%d", item.Name, item.ID)
		}
		for _, item := range resp.Items {
			strId := fmt.Sprint(item.Conversation.Peer.ID)
			conversation := map[string]string{
				"id":    strId,
				"title": "",
			}
			if item.Conversation.Peer.Type == "chat" {
				conversation["title"] = fmt.Sprintf("%s_%s", item.Conversation.ChatSettings.Title, strId)
			} else if title, ok := titles[item.Conversation.Peer.ID]; ok {
				conversation["title"] = title
			}
			result = append(result, conversation)
		}
	}
	return result, nil
}

type photoFetcher struct {
	nextPhoto int
	items     []object.PhotosPhoto
	cur       int
	albumName string
	id        string
}

func (pf *photoFetcher) Next() bool {
	pf.cur = pf.nextPhoto
	if pf.cur == len(pf.items) {
		return false
	}
	pf.nextPhoto++
	return true
}

// Downloading photos from a VK album.
func (v *Vk) AlbumPhotos(albumID string) (sources.ItemFetcher, error) {
	params := api.Params{"album_ids": albumID}
	if strings.Contains(albumID, "-") {
		params["need_system"] = 1
	}
	albumResp, err := v.vkAPI.PhotosGetAlbums(params)
	if err != nil {
		return nil, makeError(err, "DownloadAlbum failed")
	}
	var resp api.PhotosGetResponse
	items := make([]object.PhotosPhoto, 0, albumResp.Count)
	for offset := 1; offset <= albumResp.Count; offset += maxCount {
		resp, err = v.vkAPI.PhotosGet(api.Params{"album_id": albumID, "count": maxCount, "photo_sizes": 1, "offset": offset})
		if err != nil {
			log.Println("DownloadAlbum:", err)
			return nil, makeError(err, "DownloadAlbum failed")
		}
		items = append(items, resp.Items...)
	}
	if albumResp.Count < 1 {
		return nil, errors.New("no such an album")
	}
	if albumResp.Items[0].Title == "" {
		return nil, errors.New("album title is empty")
	}

	return &photoFetcher{items: items, albumName: albumResp.Items[0].Title}, nil
}

func (v *Vk) ConversationPhotos(peerId, title string) (sources.ItemFetcher, error) {
	const maxHistoryAttachments = 200
	const groupChatOffset = 2000000000
	name := title
	intPeerId, _ := strconv.Atoi(peerId)
	if len(name) < 1 && (intPeerId < 0 || intPeerId >= groupChatOffset) {
		name = peerId
	}

	params := api.Params{
		"peer_id":            peerId,
		"count":              maxHistoryAttachments,
		"photo_sizes":        1,
		"max_forwards_level": 45,
		"media_type":         "photo",
	}

	var resp api.MessagesGetHistoryAttachmentsResponse
	var err error

	items := make([]object.PhotosPhoto, 0, 1)
	for {
		resp, err = v.vkAPI.MessagesGetHistoryAttachments(params)
		if err != nil {
			log.Println("ConversationPhotos:", err)
			return nil, makeError(err, "ConversationPhotos failed")
		}
		size := len(resp.Items)
		if size < 1 {
			break
		}

		log.Printf("Getting photos from '%s': %d got", peerId, size)

		for _, item := range resp.Items {
			items = append(items, item.Attachment.Photo)
		}
		if len(name) < 1 {
			profile := resp.Profiles[len(resp.Profiles)-1]
			name = fmt.Sprintf("%s_%s_%s", profile.FirstName, profile.LastName, peerId)
		}
		params["start_from"] = resp.NextFrom

	}
	if len(items) < 1 {
		msg := fmt.Sprintf("there are no attacments in this conversation: %s", peerId)
		log.Println(msg)
		return nil, fmt.Errorf(msg)
	}

	return &photoFetcher{items: items, albumName: name, id: peerId}, nil

}

func Area(b object.PhotosPhotoSizes) float64 {
	return b.Height * b.Width
}

func (pf *photoFetcher) Item() sources.Photo {
	photo := pf.items[pf.cur]

	sort.Slice(photo.Sizes, func(i, j int) bool {
		return Area(photo.Sizes[i]) > Area(photo.Sizes[j])
	})

	urls := make([]string, 0, len(photo.Sizes))
	for _, s := range photo.Sizes {
		urls = append(urls, s.URL)
	}

	created := time.Unix(int64(photo.Date), 0)
	return &PhotoItem{
		url:       urls,
		created:   created,
		albumName: pf.albumName,
		latitude:  photo.Lat,
		longitude: photo.Long,
	}
}

func makeError(err error, text string) error {
	if errors.Is(err, api.ErrSignature) || errors.Is(err, api.ErrAccess) || errors.Is(err, api.ErrAuth) {
		return &sources.AccessError{Text: text, Err: err}
	}
	return fmt.Errorf("%s: %w", text, err)
}

type service struct{}

func (s *service) Kind() sources.Kind {
	return sources.KindSource
}

func (s *service) Key() string {
	return "vk"
}

func (s *service) Constructor() func(creds string) sources.Source {
	return New
}

func NewService() sources.ServiceSource {
	return &service{}
}
