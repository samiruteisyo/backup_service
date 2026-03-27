import { useState, useEffect } from 'react';
import './Toast.css';

export default function Toast({ message, type = 'success', duration = 3000, onClose }) {
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    const timer = setTimeout(() => {
      setVisible(false);
      setTimeout(onClose, 300);
    }, duration);
    return () => clearTimeout(timer);
  }, [duration, onClose]);

  return (
    <div className={`toast toast-${type} ${visible ? '' : 'toast-hide'}`}>
      <span className="toast-icon">
        {type === 'success' ? '✓' : type === 'error' ? '✗' : 'ℹ'}
      </span>
      <span className="toast-message">{message}</span>
      <button className="toast-close" onClick={onClose}>&times;</button>
    </div>
  );
}

export function ToastContainer({ toasts, removeToast }) {
  return (
    <div className="toast-container">
      {toasts.map((toast) => (
        <Toast
          key={toast.id}
          message={toast.message}
          type={toast.type}
          duration={toast.duration}
          onClose={() => removeToast(toast.id)}
        />
      ))}
    </div>
  );
}
