import { createRoot } from 'react-dom/client';
import React from 'react';
import App from './App';
import '@fontsource-variable/inter/opsz.css';
import '@fontsource/jetbrains-mono/400.css';
import '@fontsource/jetbrains-mono/500.css';
import './index.css';
import './theme';

createRoot(document.getElementById('app')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
