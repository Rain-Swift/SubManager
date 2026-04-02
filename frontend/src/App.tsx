import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { hasToken } from './api';

import Layout from './layout';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Subscriptions from './pages/Subscriptions';
import Rules from './pages/Rules';
import Profiles from './pages/Profiles';
import Tokens from './pages/Tokens';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!hasToken()) {
    return <Navigate to="/login" replace />;
  }
  return <Layout>{children}</Layout>;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        
        {/* Protected Routes */}
        <Route path="/" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
        <Route path="/subscriptions" element={<ProtectedRoute><Subscriptions /></ProtectedRoute>} />
        <Route path="/rules" element={<ProtectedRoute><Rules /></ProtectedRoute>} />
        <Route path="/profiles" element={<ProtectedRoute><Profiles /></ProtectedRoute>} />
        <Route path="/tokens" element={<ProtectedRoute><Tokens /></ProtectedRoute>} />
      </Routes>
    </BrowserRouter>
  );
}
