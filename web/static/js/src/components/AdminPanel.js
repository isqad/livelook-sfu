import React from 'react';
import {
  BrowserRouter,
  Routes,
  Route
} from 'react-router-dom';
import StreamShow from './StreamShow';
import Streams from './Streams';

export default function AdminPanel() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/admin" element={<Streams />} />
        <Route path="/admin/stream/:id" element={<StreamShow />} />
      </Routes>
    </BrowserRouter>
  )
}
