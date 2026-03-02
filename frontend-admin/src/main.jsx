import { createRoot } from 'react-dom/client';
import React from 'react';
import App from './App';
import './index.css';

createRoot(document.getElementById('app')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
