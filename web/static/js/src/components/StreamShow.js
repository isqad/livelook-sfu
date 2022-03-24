import React, { useEffect, useState, useRef } from 'react';
import { useParams } from "react-router-dom";

export default function StreamShow() {
  const peerConnection = new RTCPeerConnection({
    iceServers: [{
      urls: 'stun:stun.l.google.com:19302'
    }]
  });
  const ws = new WebSocket('wss://localhost:3001/api/v1/ws');
  const [isLoading, setIsLoading] = useState(false);
  const videoRef = useRef(null);
  const stream = new MediaStream();
  let { id } = useParams();

  useEffect(() => {
    if (isLoading) {
      return;
    }

    setIsLoading(true);

    ws.onopen = () => _initPC();
    ws.onmessage = _handleRpc;

    videoRef.current.srcObject = stream;
    //videoRef.current.autoplay = true;
    videoRef.current.controls = true;
    //

    return () => {
      peerConnection.close();
      ws.close();
    }
  }, []);

  const _initPC = () => {
    peerConnection.addTransceiver('video', {direction: 'recvonly'});
    peerConnection.addTransceiver('audio', {direction: 'recvonly'});

    peerConnection.oniceconnectionstatechange = (event) => {
      console.log('ICE Connection state: ', peerConnection.iceConnectionState);
    };

    peerConnection.onicegatheringstatechange = (event) => {
      const connection = event.target;

      // Now we can activate broadcast button
      if (connection.iceGatheringState === 'complete') {
        console.log('ICE gathering complete');
      }
    };

    peerConnection.onconnectionstatechange = (event) => {
      const connection = event.target;

      if (connection.connectionState === 'connected') {
        ws.send(
          JSON.stringify({
            jsonrpc: '2.0',
            method: 'add_remote_peer',
            params: {'user_id': id},
          })
        );
      }
    };

    peerConnection.createOffer().
      then((offer) => {
        peerConnection.setLocalDescription(offer).then(() => {
          fetch('https://localhost:3001/api/v1/session', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json;charset=utf-8'
            },
            body: JSON.stringify({
              sdp: peerConnection.localDescription,
            })
          }).then(response => null).
            catch(console.error);
        }).catch(console.error);
      }).catch(console.error);

    peerConnection.ontrack = (event) => {
      stream.addTrack(event.track);

      // stream.getTracks().forEach((track) => stream.removeTrack(track));

      if (event.track.kind == 'video') {
        const videoTrack = event.track;
        videoTrack.addEventListener('unmute', (event) => {
          console.log('Event video track', event);
        });

        videoTrack.addEventListener('mute', (event) => {
          console.log('Event video track', event);

        });

        videoTrack.addEventListener('ended', (event) => {
          console.log('Event video track', event);
        });

        videoRef.current.play();
      }
    };
  };

  const _handleRpc = (event) => {
    const data = JSON.parse(event.data);

    switch (data.method) {
      case 'answer':
        peerConnection.setRemoteDescription(data.params).catch(console.error);
        break;
      case 'iceCandidate':
        peerConnection.addIceCandidate({
          candidate: data.params.candidate,
          sdpMid: data.params.sdpMid,
          sdpMLineIndex: data.params.sdpMLineIndex,
        }).catch(console.error);
        break;
    }
  };

  return (
    <div>
      <div><video width="640" height="480" ref={videoRef} /></div>
    </div>
  )
}
