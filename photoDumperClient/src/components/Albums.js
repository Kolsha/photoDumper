import AlbumItem from './AlbumItem';
import { Row, Col, Typography, Badge } from 'antd';

const { Title } = Typography;

const Albums = props => {
    let rows = [];
    let row = [];
    for (let i = 0; i < props.items.length; i++) {
        row.push(<Col key={i} className="gutter-row" span={4}>
            <AlbumItem
                key={props.items[i].id}
                thumb={props.items[i].thumb}
                albumId={props.items[i].id}
                downloadAlbum={props.downloadAlbum}
                size={props.items[i].size}
                created={props.items[i].created}
                title={props.items[i].title}></AlbumItem>
        </Col>);
        if (row.length === 5) {
            rows.push(row);
            row = [];
        }
    }
    if (row.length ) {
        rows.push(row);
    }

    return (
        <>
            <Title>{props.title} albums <Badge count={props.items.length} /> </Title>
            {rows.map((item, i) =>
            (
                <Row gutter={13} key={i}>{item}</Row>
            )
            )}
        </>
    )
}

export default Albums
