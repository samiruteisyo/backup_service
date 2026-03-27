import './Button.css';

export default function Button({ children, variant = 'primary', size = 'md', loading, disabled, onClick, type = 'button', className = '' }) {
  return (
    <button
      type={type}
      className={`btn btn-${variant} btn-${size} ${loading ? 'btn-loading' : ''} ${className}`}
      disabled={disabled || loading}
      onClick={onClick}
    >
      {loading ? <span className="btn-spinner" /> : null}
      <span className={loading ? 'btn-text-hidden' : ''}>{children}</span>
    </button>
  );
}
