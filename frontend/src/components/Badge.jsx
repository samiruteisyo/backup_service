import './Badge.css';

export default function Badge({ variant = 'default', children }) {
  return <span className={`badge badge-${variant}`}>{children}</span>;
}
