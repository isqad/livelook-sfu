import React, { useEffect, useState, useRef } from 'react';
import { v4 as uuidv4 } from 'uuid';
import './Room.css';

const peerConnection = new RTCPeerConnection({
  iceServers: [{
    urls: 'stun:stun.l.google.com:19302'
  }]
});

export default function Homepage() {
  const videoRef = useRef(null);
  const [streaming, setStreaming] = useState(false);
  const [connected, setConnected] = useState(false);
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

    peerConnection.onnegotiationneeded = (event) => {
      peerConnection.createOffer().
        then((offer) => {
          peerConnection.setLocalDescription(offer).then(() => {
            fetch('https://localhost:3001/api/v1/broadcasts', {
              method: 'POST',
              headers: {
                'Content-Type': 'application/json;charset=utf-8'
              },
              body: JSON.stringify({
                user_id: userId,
                title: "Hello, pion!",
                sdp: peerConnection.localDescription,
              })
            }).then(response => null).
              catch(console.error);
          }).catch(console.error);

        }).catch(console.error);
    };
    peerConnection.onicegatheringstatechange = (event) => {
      const connection = event.target;

      // Now we can activate broadcast button
      if (connection.iceGatheringState === 'complete') {
        console.log('ICE gathering complete');
      }
    };

    console.log(peerConnection);
  };

  const _setRemoteDescription = (sdpString) => {
    peerConnection.setRemoteDescription(new RTCSessionDescription(sdpString)).then(() => {
      console.log('remote description set');
    }).catch(console.error);
  };

  const startStream = () => {
    navigator.mediaDevices.getUserMedia({video: true, audio: true}).then(stream => {
      const tracks = stream.getTracks();
      for (const track of tracks) {
        peerConnection.addTrack(track);
      }
      videoRef.current.srcObject = stream;

      setStreaming(true);
    }).catch(console.error);
  };

  return (
    <div className="container">
      <div className="row">
        <div className="col">
          <div><video ref={videoRef} width="320" height="240" autoPlay muted /></div>
          {connected &&
            <div>
              {!streaming
                ? <button type="button" className="btn btn-success" onClick={startStream}>Start stream</button>
                : <button type="button" className="btn btn-secondary" onClick={() => setStreaming(false)}>Stop stream</button>}
            </div>}
        </div>
      </div>
    </div>
  );
}
