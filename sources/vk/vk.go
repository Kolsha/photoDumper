package vk

import (
	"errors"
	"fmt"
	"log"
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
	items     []PhotoItem
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
	items := make([]PhotoItem, 0, albumResp.Count)
	for offset := 1; offset <= albumResp.Count; offset += maxCount {
		resp, err = v.vkAPI.PhotosGet(api.Params{"album_id": albumID, "count": maxCount, "photo_sizes": 1, "offset": offset})
		if err != nil {
			log.Println("DownloadAlbum:", err)
			return nil, makeError(err, "DownloadAlbum failed")
		}
		for _, item := range resp.Items {
			photo := toPhotoItem(item)
			items = append(items, photo)
		}
	}
	if albumResp.Count < 1 {
		return nil, errors.New("no such an album")
	}
	if albumResp.Items[0].Title == "" {
		return nil, errors.New("album title is empty")
	}

	return &photoFetcher{items: items, albumName: albumResp.Items[0].Title}, nil
}

func convertPhoto(a object.MessagesHistoryMessageAttachment) (PhotoItem, error) {
	return toPhotoItem(a.Photo), nil //fmt.Errorf("skip for now")
}

func convertDoc(a object.MessagesHistoryMessageAttachment) (PhotoItem, error) {
	const imageDocType = 4
	const videoDocType = 6
	result := PhotoItem{}
	if a.Type != "doc" {
		return result, fmt.Errorf("not supported type of attachment: %s", a.Type)
	}
	doc := a.Doc
	if doc.Type != imageDocType && doc.Type != videoDocType {
		return result, fmt.Errorf("not supported type of document: %d", doc.Type)
	}
	result.url = []string{doc.URL}
	result.created = time.Unix(int64(doc.Date), 0)
	result.extension = doc.Ext
	result.description = doc.Title
	result.sourceUrl = fmt.Sprintf("https://vk.com/%s", doc.ToAttachment())
	return result, nil
}

func convertVideo(a object.MessagesHistoryMessageAttachment) (PhotoItem, error) {
	result := PhotoItem{}
	if a.Type != "video" {
		return result, fmt.Errorf("not supported type of attachment: %s", a.Type)
	}
	video := a.Video
	if !video.CanDownload {
		return result, fmt.Errorf("can't download video")
	}
	files := video.Files
	urls := []string{files.Src, files.Mp4_2160, files.Mp4_1440, files.Mp4_1080, files.Mp4_720, files.Mp4_480, files.Mp4_360, files.Mp4_240, files.Mp4_144}
	result.url = make([]string, 0, 2)
	for _, url := range urls {
		if url != "" {
			result.url = append(result.url, url)
		}
	}

	result.created = time.Unix(int64(video.Date), 0)
	result.extension = "mp4"
	result.description = video.Description
	result.sourceUrl = fmt.Sprintf("https://vk.com/%s", video.ToAttachment())
	return result, nil
}

func (v *Vk) ConversationPhotos(peerId, title string) (sources.ItemFetcher, error) {
	photos, photosName, photosErr := v.ConversationAttachments(peerId, "photo", title, convertPhoto)
	videos, videosName, videosErr := v.ConversationAttachments(peerId, "video", title, convertVideo)
	docs, docsName, docsErr := v.ConversationAttachments(peerId, "doc", title, convertDoc)
	if photosErr != nil && docsErr != nil && videosErr != nil {
		return nil, photosErr
	}
	items := make([]PhotoItem, 0, len(photos)+len(videos)+len(docs))
	items = append(items, photos...)
	items = append(items, videos...)
	items = append(items, docs...)
	if len(docsName) > len(photosName) {
		photosName = docsName
	}
	if len(videosName) > len(photosName) {
		photosName = videosName
	}
	return &photoFetcher{items: items, albumName: photosName, id: peerId}, nil
}

type attachmentConverter func(object.MessagesHistoryMessageAttachment) (PhotoItem, error)

func (v *Vk) ConversationAttachments(peerId, mediaType, title string, converter attachmentConverter) ([]PhotoItem, string, error) {
	const maxHistoryAttachments = 200
	const groupChatOffset = 2000000000
	intPeerId, _ := strconv.Atoi(peerId)
	if len(title) < 1 && (intPeerId < 0 || intPeerId >= groupChatOffset) {
		title = peerId
	}

	params := api.Params{
		"peer_id":            peerId,
		"count":              maxHistoryAttachments,
		"photo_sizes":        1,
		"max_forwards_level": 45,
		"media_type":         mediaType,
	}

	var resp api.MessagesGetHistoryAttachmentsResponse
	var err error

	items := make([]PhotoItem, 0, 1)
	for {
		resp, err = v.vkAPI.MessagesGetHistoryAttachments(params)
		if err != nil {
			log.Println("ConversationAttachments:", err)
			return nil, "", makeError(err, "ConversationAttachments failed")
		}
		size := len(resp.Items)
		if size < 1 {
			break
		}

		for _, item := range resp.Items {
			photo, convertErr := converter(item.Attachment)
			if convertErr != nil {
				// log.Println("attacment skipped: ", convertErr)
				continue
			}
			items = append(items, photo)
		}
		if len(title) < 1 {
			profile := resp.Profiles[len(resp.Profiles)-1]
			title = fmt.Sprintf("%s_%s_%s", profile.FirstName, profile.LastName, peerId)
		}
		params["start_from"] = resp.NextFrom

	}
	if size := len(items); size < 1 {
		msg := fmt.Sprintf("there are no attacments in this conversation: %s", peerId)
		log.Println(msg)
		return nil, "", fmt.Errorf(msg)
	} else {
		log.Printf("Getting attacments from '%s': %d got", peerId, size)
	}

	return items, title, nil
}

func (pf *photoFetcher) Item() sources.Photo {
	photo := pf.items[pf.cur]
	photo.albumName = pf.albumName
	return &photo
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
