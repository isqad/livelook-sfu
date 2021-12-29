import React, { useEffect, useState, useRef } from 'react';
import { v4 as uuidv4 } from 'uuid';
import './Room.css';

const peerConnection = new RTCPeerConnection({
  iceServers: [{
    urls: 'stun:stun.l.google.com:19302'
  }]
});

export default function Room() {
  const videoRef = useRef(null);
  const [connected, setConnected] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [stream, setStream] = useState(new MediaStream());
  const [userId, _] = useState(uuidv4());

  useEffect(() => {
    const ws = new WebSocket('wss://localhost:3001/ws?uuid=' + userId);
    ws.onopen = () => {
      _initPC()
      setConnected(true);
    };

    ws.onclose = (event) => {
      console.error("Server closed connection abnormally: ", event.code, event.reason);
      peerConnection.close();
      setStreaming(false);
      setConnected(false);
    };

    ws.onmessage = (e) => {
      const message = JSON.parse(e.data);

      switch (message.method) {
        case "answer":
          _setRemoteDescription(message.params);
          break;
        default:
          console.error("Undefined rpc method: ", message.method);
      }
    };

    videoRef.current.srcObject = stream;
    //videoRef.current.autoplay = true;
    //videoRef.current.controls = true;

    return () => {
      peerConnection.close();
      ws.close();
    }
  }, []);

  const _initPC = () => {
    peerConnection.oniceconnectionstatechange = (event) => {
      console.log('Connection state: ', peerConnection.iceConnectionState);

      //if (peerConnection.iceConnectionState === 'connected') {
        //setBroadcasting(true);
        //setBroadcastingBtnActive(true);
      //}
    };

    peerConnection.onicegatheringstatechange = (event) => {
      const connection = event.target;

      // Now we can activate broadcast button
      if (connection.iceGatheringState === 'complete') {
        console.log('ICE gathering complete');
      }
    };

    peerConnection.addTransceiver('video');
    peerConnection.addTransceiver('audio');

    peerConnection.createOffer().
      then((offer) => {
        peerConnection.setLocalDescription(offer).then(() => {
          fetch('https://localhost:3001/api/v1/broadcasts/'+app.config.broadcastId+'/viewers', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json;charset=utf-8'
            },
            body: JSON.stringify({
              user_id: userId,
              sdp: peerConnection.localDescription,
            })
          }).then(response => null).
            catch(console.error);
        }).catch(console.error);

      }).catch(console.error);

    peerConnection.ontrack = (event) => {
      stream.addTrack(event.track);
      if (event.track.kind == 'video') {
        videoRef.current.play();
      }
    };
  };

  const _setRemoteDescription = (sdpString) => {
    peerConnection.setRemoteDescription(new RTCSessionDescription(sdpString)).then(() => {
      console.log('remote description set');
    }).catch(console.error);
  };

  return (
    <div className="container">
      <div className="row">
        <div className="col">
          <div><video width="640" height="480" ref={videoRef} /></div>
        </div>
      </div>
    </div>
  );
}
