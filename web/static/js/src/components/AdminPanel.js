import React, { useEffect, useState } from 'react';

export default function AdminPanel() {
  const [streams, setStreams] = useState(null);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    if (isLoading) {
      return;
    }

    setIsLoading(true);

    fetch(`/api/v1/streams`, {
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
    }).then(response => response.json()).then(streams => {
      setStreams(streams);
    });
  });

  return (
    <table className="table">
      <thead>
        <tr>
          <th></th>
          <th>User name</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
      {streams && streams.map(stream => {
        return (
          <tr key={stream.id}>
            <td>{stream.id}</td>
            <td>{stream.user_id}</td>
            <td></td>
          </tr>
        )
      })}
      </tbody>
    </table>
  )
}
