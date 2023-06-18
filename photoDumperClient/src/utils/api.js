let apiUrl = "http://localhost:8080/api/"
if (process.env.NODE_ENV === 'production') {
    apiUrl = '/api/'
}

const albumsApi = apiUrl + 'albums/';
const sourcesApi = apiUrl + 'sources/';
const downloadAllApi = apiUrl + 'download-all-albums/';
const downloadApi = apiUrl + 'download-album/';

const api = async (path, source = '', apiKey = '', dir = '') => {
    if (source !== '') {
        path += source + '/'
    }
    if (apiKey !== '') {
        path += '?api_key=' + apiKey
    }
    if (dir !== '') {
        path += '&dir=' + dir
    }
    const resp = await fetch(path)
    if (resp.status === 401 || resp.status === 400) {
        window.location.hash = ""
        throw new Error("please either update token or sign in again")
    }
    const data = await resp.json()
    return data
}

export const fetchAlbumsApi = async (source, apiKey) => {
    return api(albumsApi, source, apiKey)
}

export const fetchSourcesApi = async () => {
    return api(sourcesApi)
}

export const downloadAllAlbumsApi = async (source, apiKey, dir) => {
    return api(downloadAllApi, source, apiKey, dir)
}

export const downloadAlbumApi = async (albumId, source, apiKey, dir) => {
    return api(downloadApi + albumId + '/', source, apiKey, dir)
}
