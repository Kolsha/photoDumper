import { Layout, Button, Input, Row, Col, Spin, message } from 'antd';
import Albums from './components/Albums';
import Sources from './components/Sources/Sources';
import React, { useState, useEffect } from 'react';
import { fetchAlbumsApi, downloadAllAlbumsApi, downloadAlbumApi } from './utils/api';
import 'antd/dist/antd.min.css';

const { Header, Footer, Content } = Layout;

function App() {
  const [source, setSource] = useState('');
  const [albums, setAlbums] = useState([]);
  const [isLoading, setIsLoading] = useState(false);
  const [token, setToken] = useState('');
  const [dir, setDir] = useState('~/photoDumper/');

  useEffect(() => {
    if (source !== '' && token !== '') {
      fetchAlbums();
    }
  }, [source, token]);

  const fetchAlbums = () => {
    setIsLoading(true)
    fetchAlbumsApi(source, token).then(data => {
      setAlbums(data.albums);
      setIsLoading(false);
    }).catch(e => {
      setSource('')
      setAlbums([])
      setToken('')
      setIsLoading(false)
      message.error(e)
    })
  }

  const downloadAllAlbums = () => {
    downloadAllAlbumsApi(source, token, dir).then(data => {
      if (data.error !== "") {
        message.error('Error occured: '+ data.error, 5)
      } else {
        message.success('Downloading has been started, your folder: '+ data.dir, 5)
      }
    }).catch(e => {
      setSource('')
      setAlbums([])
      setToken('')
      setIsLoading(false)
      message.error('Downloading has failed: ' + e)
    })
  }
  const downloadAlbum = albumId => {
    downloadAlbumApi(albumId, source, token, dir).then(data => {
      if (data.error !== "") {
        message.error('Error occured: '+ data.error, 5)
      } else {
        message.success('Downloading has been started, your folder: '+ data.dir, 5)
      }
    }).catch(e => {
      setSource('')
      setAlbums([])
      setToken('')
      setIsLoading(false)
      message.error('Downloading has failed: ' + e, 5)
    })
  }
  return (
    <>
      <Layout>
        <Header>
          {source && (
            <Row gutter={8}>
              <Col span={3}>
                <Button type="primary" onClick={downloadAllAlbums}>Download all albums</Button>
              </Col>
              <Col span={8}>
                <Input type="text" placeholder="path" value={dir} onChange={event => { setDir(event.target.value) }} />
              </Col>
            </Row>
          )}
        </Header>
        <Content style={{ paddingLeft: 50 }}>
          {!source && <div>
            <Sources setSource={setSource} setToken={setToken}></Sources>
          </div>}
          {!isLoading && albums.length > 0 && <Albums items={albums} downloadAlbum={downloadAlbum} title={source}></Albums>}
          {!isLoading && source && albums.length === 0 && <p>No Albums</p>}
          {isLoading && <Spin />}
        </Content>
        <Footer></Footer>
      </Layout>
    </>

  );
}

export default App;
