import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClientProvider } from '@tanstack/react-query';
import { createQueryClient } from './app/query-client';
import { App } from './app/app';
import './styles/globals.css';

const root = document.getElementById('root');
if (!root) throw new Error('页面缺少应用挂载节点。');

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={createQueryClient()}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
);
