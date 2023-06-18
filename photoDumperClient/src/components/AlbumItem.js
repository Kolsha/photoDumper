import { Card, Button } from 'antd';
import { DownloadOutlined } from '@ant-design/icons';
import React, { useState } from 'react';

const { Meta } = Card;

const AlbumItem = props => {
    const [isShowButton, setIsShowButton] = useState(false);
    return (
        <Card
            onMouseEnter={() => { setIsShowButton(true) }}
            onMouseLeave={() => { setIsShowButton(false) }}
            hoverable
            style={{ width: 240, height: 320 }}
            cover={<img alt={props.title} src={props.thumb ? props.thumb : "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="} style={{ maxHeight: 200, width: 'auto', margin: 30 }} />}
        >
            <Meta title={props.title} />
            <p>{props.size} elements</p>
            {isShowButton && <Button type="default" style={{ marginTop: 10 }} onClick={event => { props.downloadAlbum(props.albumId) }} icon={<DownloadOutlined />}>Download</Button>}
        </Card>
    )
}

export default AlbumItem