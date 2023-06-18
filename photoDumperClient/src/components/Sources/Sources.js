import React, { useEffect } from 'react';
// import { fetchSourcesApi } from '../../utils/api';
import vk from './vk.jpg';
import instagram from './instagram.png';
import './Sources.css';
const clientId = '8145193';

const oauthVkUrl = `https://oauth.vk.com/authorize?client_id=${clientId}&display=page&scope=photos&response_type=token&v=5.131&state=vk&redirect_uri=${window.location.href}`
const oauthInstagramUrl = `https://api.instagram.com/oauth/authorize?client_id=750539649411327&redirect_uri=https://insta-auth-gjn33lyilq-ey.a.run.app/insta/&scope=user_profile,user_media&response_type=code`

const getFragment = fragment => {
    let fragments = window.location.hash.substring(1).split('&')
    let val = ""
    fragments.forEach(item => {
        let el = item.split('=')
        if (el[0] === fragment) {
            val = el[1]
            return
        }
    })
    return val
}

const Sources = props => {
    useEffect(() => {
        const token = getFragment('access_token')
        const state = getFragment('state')
        if (state !== '' && token !== '') {
            props.setToken(token)
            props.setSource(state)
            console.log('state:', state)
        }
    }, []);
    return (
        <div className='sources'>
            <p>Please choose social net and then click on icon in order to authroize.</p>
            <a href={ oauthVkUrl }><img src={vk} alt='vk logo' /> </a>
            <a href={ oauthInstagramUrl }><img src={instagram} alt='instagram logo' /> </a>
        </div>
    )
}
export default Sources