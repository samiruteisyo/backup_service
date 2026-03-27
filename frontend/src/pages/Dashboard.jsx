import { useState, useEffect, useCallback } from 'react';
import { api } from '../api/client';
import { useAuth } from '../hooks';
import { Button, Card, CardBody, Badge, Tabs, Modal } from '../components';
import './Dashboard.css';

function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatDate(dateStr) {
  if (!dateStr) return 'Never';
  const date = new Date(dateStr);
  const now = new Date();
  const diff = now - date;
  const mins = Math.floor(diff / 60000);
  const hours = Math.floor(diff / 3600000);
  const days = Math.floor(diff / 86400000);

  if (mins < 1) return 'Just now';
  if (mins < 60) return `${mins}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days < 7) return `${days}d ago`;
  return date.toLocaleDateString();
}

function formatFullDate(dateStr) {
  if (!dateStr) return 'Never';
  return new Date(dateStr).toLocaleString();
}

function shortSha(sha) {
  return sha ? sha.substring(0, 7) : '-';
}

export default function Dashboard({ showToast }) {
  const { logout } = useAuth();
  const [projects, setProjects] = useState([]);
  const [projectDetails, setProjectDetails] = useState({});
  const [expandedProjects, setExpandedProjects] = useState({});
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState({});
  const [confirmModal, setConfirmModal] = useState(null);

  const loadProjects = useCallback(async () => {
    try {
      const data = await api.getProjects();
      setProjects(Array.isArray(data) ? data : (data.projects || []));
    } catch (err) {
      showToast('Failed to load projects', 'error');
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  const loadProjectDetail = useCallback(async (name) => {
    try {
      const data = await api.getProject(name);
      setProjectDetails((prev) => ({ ...prev, [name]: data }));
    } catch (err) {
      showToast(`Failed to load ${name}`, 'error');
    }
  }, [showToast]);

  useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  const toggleProject = (name) => {
    setExpandedProjects((prev) => {
      const next = { ...prev, [name]: !prev[name] };
      if (next[name] && !projectDetails[name]) {
        loadProjectDetail(name);
      }
      return next;
    });
  };

  const handleBackup = async (name) => {
    setActionLoading((prev) => ({ ...prev, [`${name}-backup`]: true }));
    try {
      await api.backupProject(name);
      showToast(`${name} backup started`, 'success');
      await loadProjectDetail(name);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setActionLoading((prev) => ({ ...prev, [`${name}-backup`]: false }));
    }
  };

  const handleDeploy = async (name) => {
    setActionLoading((prev) => ({ ...prev, [`${name}-deploy`]: true }));
    try {
      await api.deployProject(name);
      showToast(`Deploying ${name}...`, 'success');
      await loadProjectDetail(name);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setActionLoading((prev) => ({ ...prev, [`${name}-deploy`]: false }));
    }
  };

  const handleRestore = async (name, timestamp) => {
    setConfirmModal(null);
    setActionLoading((prev) => ({ ...prev, [`${name}-restore`]: true }));
    try {
      await api.restoreProject(name, timestamp);
      showToast(`Restoring ${name}...`, 'success');
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setActionLoading((prev) => ({ ...prev, [`${name}-restore`]: false }));
    }
  };

  const handleDeleteBackup = async (name, timestamp) => {
    setConfirmModal(null);
    setActionLoading((prev) => ({ ...prev, [`${name}-delete`]: true }));
    try {
      await api.deleteBackup(name, timestamp);
      showToast('Backup deleted', 'success');
      await loadProjectDetail(name);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setActionLoading((prev) => ({ ...prev, [`${name}-delete`]: false }));
    }
  };

  const handleRollback = async (name, sha) => {
    setConfirmModal(null);
    setActionLoading((prev) => ({ ...prev, [`${name}-rollback`]: true }));
    try {
      await api.rollbackProject(name, sha);
      showToast(`Rolling back ${name}...`, 'success');
      await loadProjectDetail(name);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setActionLoading((prev) => ({ ...prev, [`${name}-rollback`]: false }));
    }
  };

  const handleBackupAll = async () => {
    setActionLoading((prev) => ({ ...prev, backupAll: true }));
    try {
      for (const p of projects) {
        await api.backupProject(p.name);
      }
      showToast('All backups started', 'success');
      await loadProjects();
      for (const p of projects) {
        await loadProjectDetail(p.name);
      }
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setActionLoading((prev) => ({ ...prev, backupAll: false }));
    }
  };

  if (loading) {
    return <div className="dashboard-loading">Loading...</div>;
  }

  const totalBackups = projects.reduce((sum, p) => sum + (p.backup_count || 0), 0);
  const totalSize = projects.reduce((sum, p) => sum + (p.total_size || 0), 0);

  return (
    <div className="dashboard">
      <header className="dashboard-header">
        <h1>Backup Service</h1>
        <div className="header-actions">
          <Button onClick={handleBackupAll} loading={actionLoading.backupAll}>
            Run All Backups
          </Button>
          <Button variant="ghost" onClick={logout}>Logout</Button>
        </div>
      </header>

      <div className="stats-row">
        <Card><CardBody><div className="stat-card">
          <div className="stat-value">{projects.length}</div>
          <div className="stat-label">Projects</div>
        </div></CardBody></Card>
        <Card><CardBody><div className="stat-card">
          <div className="stat-value">{totalBackups}</div>
          <div className="stat-label">Total Backups</div>
        </div></CardBody></Card>
        <Card><CardBody><div className="stat-card">
          <div className="stat-value">{formatBytes(totalSize)}</div>
          <div className="stat-label">Total Size</div>
        </div></CardBody></Card>
      </div>

      <div className="project-list">
        {projects.length === 0 ? (
          <div className="empty-state">No projects found</div>
        ) : (
          projects.map((project) => (
            <ProjectCard
              key={project.name}
              project={project}
              detail={projectDetails[project.name]}
              expanded={expandedProjects[project.name]}
              onToggle={() => toggleProject(project.name)}
              actionLoading={actionLoading}
              onBackup={handleBackup}
              onDeploy={handleDeploy}
              onRestore={(ts) => setConfirmModal({ type: 'restore', name: project.name, timestamp: ts })}
              onDelete={(ts) => setConfirmModal({ type: 'delete', name: project.name, timestamp: ts })}
              onRollback={(sha) => setConfirmModal({ type: 'rollback', name: project.name, sha })}
            />
          ))
        )}
      </div>

      {confirmModal && (
        <ConfirmModal
          type={confirmModal.type}
          name={confirmModal.name}
          timestamp={confirmModal.timestamp}
          sha={confirmModal.sha}
          onConfirm={() => {
            if (confirmModal.type === 'restore') handleRestore(confirmModal.name, confirmModal.timestamp);
            if (confirmModal.type === 'delete') handleDeleteBackup(confirmModal.name, confirmModal.timestamp);
            if (confirmModal.type === 'rollback') handleRollback(confirmModal.name, confirmModal.sha);
          }}
          onCancel={() => setConfirmModal(null)}
        />
      )}
    </div>
  );
}

function ProjectCard({ project, detail, expanded, onToggle, actionLoading, onBackup, onDeploy, onRestore, onDelete, onRollback }) {
  const tabs = [
    {
      label: 'Backups',
      value: 'backups',
      children: detail ? (
        <BackupList
          backups={detail.backups}
          loading={actionLoading[`${project.name}-delete`]}
          onRestore={onRestore}
          onDelete={onDelete}
        />
      ) : (
        <div className="tab-loading">Loading...</div>
      ),
    },
    {
      label: 'Deployments',
      value: 'deployments',
      children: detail ? (
        <DeploymentList
          deployments={detail.deployments}
          onRollback={onRollback}
        />
      ) : (
        <div className="tab-loading">Loading...</div>
      ),
    },
    {
      label: 'Activity',
      value: 'activity',
      children: detail ? (
        <ActivityLog activities={detail.activity} />
      ) : (
        <div className="tab-loading">Loading...</div>
      ),
    },
  ];

  return (
    <Card className={`project-card ${expanded ? 'expanded' : ''}`}>
      <CardBody>
        <div className="project-header" onClick={onToggle}>
          <div className="project-info">
            <div className="project-name-row">
              <span className="project-name">{project.name}</span>
              {project.db_type && <Badge variant="info">{project.db_type}</Badge>}
              {project.branch && <Badge>{project.branch}</Badge>}
              {project.commits_behind > 0 && (
                <Badge variant="warning">{project.commits_behind} commits behind</Badge>
              )}
            </div>
            <div className="project-meta">
              <span>{project.backup_count || 0} backups</span>
              <span>•</span>
              <span>{formatBytes(project.total_size || 0)}</span>
              <span>•</span>
              <span>Last: {formatDate(project.last_backup)}</span>
            </div>
          </div>
          <div className="project-actions" onClick={(e) => e.stopPropagation()}>
            <Button
              size="sm"
              variant="success"
              loading={actionLoading[`${project.name}-backup`]}
              onClick={() => onBackup(project.name)}
            >
              Backup Now
            </Button>
            <Button
              size="sm"
              variant="primary"
              loading={actionLoading[`${project.name}-deploy`]}
              onClick={() => onDeploy(project.name)}
            >
              Deploy
            </Button>
            <span className={`expand-icon ${expanded ? 'expanded' : ''}`}>▼</span>
          </div>
        </div>

        {expanded && (
          <div className="project-detail">
            <Tabs tabs={tabs} defaultTab="backups" />
          </div>
        )}
      </CardBody>
    </Card>
  );
}

function BackupList({ backups, loading, onRestore, onDelete }) {
  if (!backups || backups.length === 0) {
    return <div className="empty-tab">No backups yet</div>;
  }

  return (
    <table className="data-table">
      <thead>
        <tr>
          <th>Timestamp</th>
          <th>Size</th>
          <th>Git SHA</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {backups.map((b) => (
          <tr key={b.timestamp}>
            <td title={formatFullDate(b.timestamp)}>{formatDate(b.timestamp)}</td>
            <td>{formatBytes(b.size)}</td>
            <td><code>{shortSha(b.sha)}</code></td>
            <td>
              <Button size="sm" variant="ghost" loading={loading} onClick={() => onRestore(b.timestamp)}>
                Restore
              </Button>
              <Button size="sm" variant="ghost" loading={loading} onClick={() => onDelete(b.timestamp)}>
                Delete
              </Button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function DeploymentList({ deployments, onRollback }) {
  if (!deployments || deployments.length === 0) {
    return <div className="empty-tab">No deployments yet</div>;
  }

  return (
    <table className="data-table">
      <thead>
        <tr>
          <th>Time</th>
          <th>Git SHA</th>
          <th>Branch</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {deployments.map((d) => (
          <tr key={d.timestamp}>
            <td title={formatFullDate(d.timestamp)}>{formatDate(d.timestamp)}</td>
            <td><code>{shortSha(d.sha)}</code></td>
            <td>{d.branch}</td>
            <td>
              <Button size="sm" variant="ghost" onClick={() => onRollback(d.sha)}>
                Rollback
              </Button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function ActivityLog({ activities }) {
  if (!activities || activities.length === 0) {
    return <div className="empty-tab">No activity yet</div>;
  }

  return (
    <div className="activity-list">
      {activities.map((a, i) => (
        <div key={i} className="activity-item">
          <span className={`activity-icon ${a.type}`}>
            {a.type === 'backup' ? '💾' : a.type === 'deploy' ? '🚀' : a.type === 'restore' ? '♻️' : '📝'}
          </span>
          <span className="activity-text">{a.message}</span>
          <span className="activity-time">{formatDate(a.timestamp)}</span>
        </div>
      ))}
    </div>
  );
}

function ConfirmModal({ type, name, timestamp, sha, onConfirm, onCancel }) {
  const titles = {
    restore: 'Confirm Restore',
    delete: 'Confirm Delete',
    rollback: 'Confirm Rollback',
  };

  const messages = {
    restore: `Restore ${name} from backup ${formatDate(timestamp)}? This will overwrite current data.`,
    delete: `Delete backup from ${formatDate(timestamp)}? This cannot be undone.`,
    rollback: `Rollback ${name} to ${shortSha(sha)}? This will discard current changes.`,
  };

  const confirmLabel = { restore: 'Restore', delete: 'Delete', rollback: 'Rollback' };

  return (
    <Modal
      isOpen={true}
      onClose={onCancel}
      title={titles[type]}
      footer={
        <>
          <Button variant="ghost" onClick={onCancel}>Cancel</Button>
          <Button variant={type === 'delete' ? 'danger' : 'primary'} onClick={onConfirm}>
            {confirmLabel[type]}
          </Button>
        </>
      }
    >
      <p>{messages[type]}</p>
    </Modal>
  );
}
