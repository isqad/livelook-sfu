import React, { useEffect, useState } from 'react';
import './Room.css';

export default function Homepage() {
  useEffect(() => {
    console.log('Started');

    const ws = new WebSocket('wss://localhost:3001/ws');
    ws.onopen = () => {};
    ws.onclose = (event) => {
      console.error("Server closed connection abnormally: ", event.code, event.reason);
    };

    ws.onmessage = (e) => {
      console.log("Got message: ", e);
    };

    return () => {
      ws.close();
    }
  }, []);

  return (
    <div className="container">
      <div className="row">
        <div className="col">
          <div><video width="320" height="240" autoPlay muted /></div>
          <div>
            <button type="button" className="btn btn-success">Start stream</button>
          </div>
        </div>
      </div>
    </div>
  );
}
