import { useState, useCallback } from 'react';
import { AuthProvider, useAuth } from './hooks';
import { ToastContainer } from './components';
import { Login, Dashboard } from './pages';

function AppContent() {
  const { user, loading } = useAuth();
  const [toasts, setToasts] = useState([]);

  const showToast = useCallback((message, type = 'success', duration = 3000) => {
    const id = Date.now();
    setToasts((prev) => [...prev, { id, message, type, duration }]);
  }, []);

  const removeToast = useCallback((id) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  if (loading) {
    return (
      <div className="app-loading">
        <div className="app-spinner" />
      </div>
    );
  }

  if (!user) {
    return <Login />;
  }

  return (
    <>
      <Dashboard showToast={showToast} />
      <ToastContainer toasts={toasts} removeToast={removeToast} />
    </>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  );
}
