import { FormEvent, useEffect, useState } from "react";

import {
  createAgent,
  createConnector,
  createWorkspace,
  deleteAgent,
  deleteConnector,
  deleteWorkspace,
  listConnectors,
  listWorkspaces,
  updateAgent,
  updateConnector,
  updateWorkspace,
} from "./api/client";
import CopyButton from "./components/CopyButton";
import type { AgentSummary, ConnectorSummary, WorkspaceSummary } from "./types";

// Navigation sections - True hierarchy:
// Company (Workspace) → Branch (Remote Network) → Connector → Agent
type View = "companies" | "networks" | "connectors" | "agents";

export default function App() {
  // Navigation state
  const [currentView, setCurrentView] = useState<View>("companies");
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState<string>("");
  const [selectedNetworkId, setSelectedNetworkId] = useState<string>("");
  const [selectedConnectorId, setSelectedConnectorId] = useState<string>("");
  
  // Data state
  const [workspaces, setWorkspaces] = useState<WorkspaceSummary[]>([]);
  const [connectors, setConnectors] = useState<ConnectorSummary[]>([]);
  const [status, setStatus] = useState<string>("Initializing...");
  const [error, setError] = useState<string>("");

  // Form states
  const [workspaceName, setWorkspaceName] = useState("");
  const [networkName, setNetworkName] = useState("");
  const [connectorName, setConnectorName] = useState("");
  const [connectorPublicAddr, setConnectorPublicAddr] = useState("");
  const [dataplaneListenAddr, setDataplaneListenAddr] = useState("0.0.0.0:9443");
  const [agentName, setAgentName] = useState("");

  // Load data
  useEffect(() => {
    void loadWorkspaces();
  }, []);

  useEffect(() => {
    if (selectedWorkspaceId) {
      void loadConnectors(selectedWorkspaceId);
    }
  }, [selectedWorkspaceId]);

  // Auto-refresh
  useEffect(() => {
    if (!selectedWorkspaceId) return;
    const timer = window.setInterval(() => {
      void loadConnectors(selectedWorkspaceId);
    }, 15000);
    return () => window.clearInterval(timer);
  }, [selectedWorkspaceId]);

  async function loadWorkspaces(): Promise<void> {
    try {
      const items = await listWorkspaces();
      setWorkspaces(items);
      setStatus("Ready");
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load companies");
      setStatus("Error");
    }
  }

  async function loadConnectors(workspaceId: string): Promise<void> {
    try {
      const items = await listConnectors(workspaceId);
      setConnectors(items);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load connectors");
    }
  }

  // Workspace (Company) handlers
  async function onCreateWorkspace(e: FormEvent): Promise<void> {
    e.preventDefault();
    if (!workspaceName.trim()) return;
    try {
      await createWorkspace(workspaceName.trim());
      setWorkspaceName("");
      await loadWorkspaces();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create company");
    }
  }

  async function onDeleteWorkspace(id: string, name: string): Promise<void> {
    if (!window.confirm(`Delete company "${name}" and all its networks/connectors/agents?`)) return;
    try {
      await deleteWorkspace(id);
      if (selectedWorkspaceId === id) {
        setSelectedWorkspaceId("");
        setCurrentView("companies");
      }
      await loadWorkspaces();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete company");
    }
  }

  // Select company and show its remote networks
  function selectWorkspace(id: string) {
    setSelectedWorkspaceId(id);
    setCurrentView("networks");
    setSelectedNetworkId("");
    setSelectedConnectorId("");
  }

  // Select network and show its connectors
  function selectNetwork(id: string) {
    setSelectedNetworkId(id);
    setCurrentView("connectors");
    setSelectedConnectorId("");
  }

  // Select connector and show its agents
  function selectConnector(id: string) {
    setSelectedConnectorId(id);
    setCurrentView("agents");
  }

  // Connector handlers
  async function onCreateConnector(e: FormEvent): Promise<void> {
    e.preventDefault();
    if (!selectedWorkspaceId || !connectorName.trim()) return;
    try {
      await createConnector({
        workspaceId: selectedWorkspaceId,
        name: connectorName.trim(),
        connectorPublicAddr: connectorPublicAddr.trim(),
        dataplaneListenAddr: dataplaneListenAddr.trim(),
      });
      setConnectorName("");
      setConnectorPublicAddr("");
      await loadConnectors(selectedWorkspaceId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create connector");
    }
  }

  async function onDeleteConnector(connector: ConnectorSummary): Promise<void> {
    if (!window.confirm(`Delete connector "${connector.name}" and all its agents?`)) return;
    try {
      await deleteConnector(connector.workspaceId, connector.id);
      if (selectedConnectorId === connector.id) {
        setSelectedConnectorId("");
      }
      await loadConnectors(selectedWorkspaceId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete connector");
    }
  }

  // Agent handlers
  async function onCreateAgent(e: FormEvent, connectorId: string): Promise<void> {
    e.preventDefault();
    if (!agentName.trim()) return;
    try {
      await createAgent(selectedWorkspaceId, connectorId, agentName.trim());
      setAgentName("");
      await loadConnectors(selectedWorkspaceId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create agent");
    }
  }

  async function onDeleteAgent(connectorId: string, agent: AgentSummary): Promise<void> {
    if (!window.confirm(`Delete agent "${agent.name}"?`)) return;
    try {
      await deleteAgent(selectedWorkspaceId, connectorId, agent.id);
      await loadConnectors(selectedWorkspaceId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete agent");
    }
  }

  // Get selected objects
  const selectedWorkspace = workspaces.find(w => w.id === selectedWorkspaceId);
  const selectedConnector = connectors.find(c => c.id === selectedConnectorId);

  // Mock networks for now - in production these would come from API
  const mockNetworks = selectedWorkspace ? [
    { id: "default", name: "Default Network", description: "Main branch/location" }
  ] : [];
  const selectedNetwork = mockNetworks.find(n => n.id === selectedNetworkId);

  return (
    <div className="app-container">
      {/* Sidebar Navigation */}
      <aside className="sidebar">
        <div className="sidebar-header">
          <h1>ZTNA</h1>
          <span className="status-dot"></span>
        </div>
        
        <nav className="sidebar-nav">
          <button 
            className={`nav-item ${currentView === "companies" ? "active" : ""}`}
            onClick={() => setCurrentView("companies")}
          >
            <span className="nav-icon">🏢</span>
            <span>Companies</span>
            <span className="nav-count">{workspaces.length}</span>
          </button>
          
          {selectedWorkspace && (
            <button 
              className={`nav-item ${currentView === "networks" ? "active" : ""}`}
              onClick={() => setCurrentView("networks")}
            >
              <span className="nav-icon">🌐</span>
              <span>Remote Networks</span>
              <span className="nav-count">1</span>
            </button>
          )}
          
          {selectedNetwork && (
            <button 
              className={`nav-item ${currentView === "connectors" ? "active" : ""}`}
              onClick={() => setCurrentView("connectors")}
            >
              <span className="nav-icon">🔌</span>
              <span>Connectors</span>
              <span className="nav-count">{selectedWorkspace?.connectorCount || 0}</span>
            </button>
          )}
          
          {selectedConnector && (
            <button 
              className={`nav-item ${currentView === "agents" ? "active" : ""}`}
              onClick={() => setCurrentView("agents")}
            >
              <span className="nav-icon">💻</span>
              <span>Agents</span>
              <span className="nav-count">{selectedConnector?.agentCount || 0}</span>
            </button>
          )}
        </nav>

        <div className="sidebar-footer">
          <div className="status">
            <span className={`status-indicator ${status === "Ready" ? "online" : "offline"}`}></span>
            <span>{status}</span>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="main-content">
        {error && <div className="error-banner">{error}</div>}

        {/* Companies (Workspaces) View */}
        {currentView === "companies" && (
          <div className="view">
            <header className="view-header">
              <h2>Companies</h2>
              <p>Manage separate company boundaries. Each company has its own isolated networks and CA.</p>
            </header>

            <section className="card create-form">
              <h3>Create Company</h3>
              <form onSubmit={onCreateWorkspace}>
                <div className="form-row">
                  <input
                    value={workspaceName}
                    onChange={e => setWorkspaceName(e.target.value)}
                    placeholder="Company name (e.g., Acme Corp)"
                    className="input"
                  />
                  <button type="submit" className="btn-primary">Create</button>
                </div>
              </form>
            </section>

            <section className="companies-list">
              {workspaces.length === 0 ? (
                <div className="empty-state">
                  <p>No companies configured</p>
                  <p>Create a company workspace to get started</p>
                </div>
              ) : (
                workspaces.map(workspace => (
                  <div 
                    key={workspace.id} 
                    className={`company-card ${selectedWorkspaceId === workspace.id ? "selected" : ""}`}
                    onClick={() => selectWorkspace(workspace.id)}
                  >
                    <div className="company-info">
                      <h4>{workspace.displayName}</h4>
                      <code>{workspace.id.slice(0, 8)}...</code>
                    </div>
                    <div className="company-stats">
                      <span>1 network</span>
                      <span>{workspace.connectorCount} connectors</span>
                      <span>{workspace.agentCount} agents</span>
                    </div>
                    <button 
                      className="btn-icon delete"
                      onClick={e => { e.stopPropagation(); void onDeleteWorkspace(workspace.id, workspace.displayName); }}
                    >
                      🗑️
                    </button>
                  </div>
                ))
              )}
            </section>
          </div>
        )}

        {/* Remote Networks View */}
        {currentView === "networks" && selectedWorkspace && (
          <div className="view">
            <header className="view-header">
              <div className="breadcrumb">
                <button onClick={() => setCurrentView("companies")}>Companies</button>
                <span>/</span>
                <span>{selectedWorkspace.displayName}</span>
              </div>
              <h2>Remote Networks</h2>
              <p>Branches, offices, or locations within {selectedWorkspace.displayName}</p>
            </header>

            <section className="card create-form">
              <h3>Create Remote Network</h3>
              <div className="form-row">
                <input
                  value={networkName}
                  onChange={e => setNetworkName(e.target.value)}
                  placeholder="Network name (e.g., HQ, Branch Office, Data Center)"
                  className="input"
                />
                <button className="btn-primary" onClick={() => {}}>Create</button>
              </div>
              <p className="form-hint">Remote networks represent physical locations where connectors will be deployed.</p>
            </section>

            <section className="networks-list">
              <div 
                className={`network-card ${selectedNetworkId === "default" ? "selected" : ""}`}
                onClick={() => selectNetwork("default")}
              >
                <div className="network-info">
                  <h4>Default Network</h4>
                  <p>Main branch/location</p>
                </div>
                <div className="network-stats">
                  <span>{selectedWorkspace.connectorCount} connectors</span>
                  <span>{selectedWorkspace.agentCount} agents</span>
                </div>
                <button className="btn-secondary">Manage</button>
              </div>
            </section>
          </div>
        )}

        {/* Connectors View */}
        {currentView === "connectors" && selectedWorkspace && selectedNetwork && (
          <div className="view">
            <header className="view-header">
              <div className="breadcrumb">
                <button onClick={() => setCurrentView("companies")}>Companies</button>
                <span>/</span>
                <button onClick={() => setCurrentView("networks")}>{selectedWorkspace.displayName}</button>
                <span>/</span>
                <span>{selectedNetwork.name}</span>
              </div>
              <h2>Connectors</h2>
              <p>Deploy connectors in this network to enable agent connections</p>
            </header>

            <section className="card create-form">
              <h3>Create Connector</h3>
              <form onSubmit={onCreateConnector}>
                <div className="form-grid">
                  <input
                    value={connectorName}
                    onChange={e => setConnectorName(e.target.value)}
                    placeholder="Connector name (e.g., office-gateway)"
                    className="input"
                  />
                  <input
                    value={connectorPublicAddr}
                    onChange={e => setConnectorPublicAddr(e.target.value)}
                    placeholder="Public address (e.g., 192.168.1.100:9443)"
                    className="input"
                  />
                  <input
                    value={dataplaneListenAddr}
                    onChange={e => setDataplaneListenAddr(e.target.value)}
                    placeholder="Listen address (0.0.0.0:9443)"
                    className="input"
                  />
                  <button type="submit" className="btn-primary">Create</button>
                </div>
              </form>
            </section>

            <section className="connectors-list">
              {connectors.length === 0 ? (
                <div className="empty-state">
                  <p>No connectors in this network</p>
                  <p>Create a connector to deploy on your network edge</p>
                </div>
              ) : (
                connectors.map(connector => (
                  <div key={connector.id} className="connector-card">
                    <div className="connector-header" onClick={() => selectConnector(connector.id)}>
                      <div className="connector-status">
                        <StatusBadge status={connector.status} />
                      </div>
                      <div className="connector-info">
                        <h4>{connector.name}</h4>
                        <p>{connector.connectorPublicAddr}</p>
                      </div>
                      <div className="connector-actions">
                        <span className="agent-count">{connector.agentCount} agents</span>
                        <button className="btn-secondary">Manage</button>
                        <button 
                          className="btn-icon delete"
                          onClick={e => { e.stopPropagation(); void onDeleteConnector(connector); }}
                        >
                          🗑️
                        </button>
                      </div>
                    </div>

                    {selectedConnectorId === connector.id && (
                      <div className="connector-details">
                        <div className="install-section">
                          <h5>Installation</h5>
                          <code className="install-command">{connector.installCurlCommand}</code>
                          <div className="copy-actions">
                            <CopyButton value={connector.installCurlCommand} label="Copy Command" />
                            <CopyButton value={connector.bootstrapToken} label="Copy Token" />
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                ))
              )}
            </section>
          </div>
        )}

        {/* Agents View */}
        {currentView === "agents" && selectedWorkspace && selectedNetwork && selectedConnector && (
          <div className="view">
            <header className="view-header">
              <div className="breadcrumb">
                <button onClick={() => setCurrentView("companies")}>Companies</button>
                <span>/</span>
                <button onClick={() => setCurrentView("networks")}>{selectedWorkspace.displayName}</button>
                <span>/</span>
                <button onClick={() => setCurrentView("connectors")}>{selectedNetwork.name}</button>
                <span>/</span>
                <span>{selectedConnector.name}</span>
              </div>
              <h2>Agents</h2>
              <p>Deploy agents on devices that need secure network access</p>
            </header>

            <section className="card create-form">
              <h3>Create Agent</h3>
              <form onSubmit={e => onCreateAgent(e, selectedConnector.id)}>
                <div className="form-row">
                  <input
                    value={agentName}
                    onChange={e => setAgentName(e.target.value)}
                    placeholder="Agent name (e.g., laptop-john)"
                    className="input"
                  />
                  <button type="submit" className="btn-primary">Create</button>
                </div>
              </form>
            </section>

            <section className="agents-list">
              {selectedConnector.agents.length === 0 ? (
                <div className="empty-state">
                  <p>No agents connected to this connector</p>
                  <p>Create an agent and install it on a client device</p>
                </div>
              ) : (
                selectedConnector.agents.map(agent => (
                  <div key={agent.id} className="agent-card">
                    <div className="agent-info">
                      <StatusBadge status={agent.status} />
                      <h4>{agent.name}</h4>
                      <code>{agent.id.slice(0, 8)}...</code>
                    </div>
                    <div className="agent-actions">
                      <CopyButton value={agent.installCurlCommand} label="Install Command" />
                      <button 
                        className="btn-icon delete"
                        onClick={() => void onDeleteAgent(selectedConnector.id, agent)}
                      >
                        🗑️
                      </button>
                    </div>
                  </div>
                ))
              )}
            </section>
          </div>
        )}
      </main>
    </div>
  );
}

function StatusBadge({ status }: { status: { verified: boolean; active: boolean; deviceStatus: string } }) {
  let className = "status-badge";
  let label = status.deviceStatus;

  if (status.active) {
    className += " active";
    label = "Active";
  } else if (status.verified) {
    className += " verified";
    label = "Verified";
  } else {
    className += " pending";
    label = "Pending";
  }

  return <span className={className}>{label}</span>;
}
