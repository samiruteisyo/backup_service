import { useState } from 'react';
import './Tabs.css';

export default function Tabs({ tabs, defaultTab, onChange }) {
  const [activeTab, setActiveTab] = useState(defaultTab || tabs[0]?.value);

  const handleTabClick = (tab) => {
    setActiveTab(tab.value);
    onChange?.(tab.value);
  };

  return (
    <div className="tabs">
      <div className="tabs-list">
        {tabs.map((tab) => (
          <button
            key={tab.value}
            className={`tab ${activeTab === tab.value ? 'tab-active' : ''}`}
            onClick={() => handleTabClick(tab)}
          >
            {tab.label}
          </button>
        ))}
      </div>
      <div className="tab-content">
        {tabs.find((t) => t.value === activeTab)?.children}
      </div>
    </div>
  );
}
