import { FormEvent, useEffect, useMemo, useState } from "react";

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

interface ConnectorDraft {
  name: string;
  connectorPublicAddr: string;
  dataplaneListenAddr: string;
}

export default function App() {
  const [workspaces, setWorkspaces] = useState<WorkspaceSummary[]>([]);
  const [selectedWorkspaceId, setSelectedWorkspaceId] = useState<string>("");
  const [connectors, setConnectors] = useState<ConnectorSummary[]>([]);
  const [status, setStatus] = useState<string>("Initializing console...");
  const [error, setError] = useState<string>("");

  const [workspaceName, setWorkspaceName] = useState("");
  const [workspaceRename, setWorkspaceRename] = useState("");

  const [connectorName, setConnectorName] = useState("");
  const [connectorPublicAddr, setConnectorPublicAddr] = useState("");
  const [dataplaneListenAddr, setDataplaneListenAddr] = useState("0.0.0.0:9443");

  const [agentNames, setAgentNames] = useState<Record<string, string>>({});
  const [connectorDrafts, setConnectorDrafts] = useState<Record<string, ConnectorDraft>>({});
  const [agentDraftNames, setAgentDraftNames] = useState<Record<string, string>>({});

  const selectedWorkspace = useMemo(
    () => workspaces.find((workspace) => workspace.id === selectedWorkspaceId),
    [workspaces, selectedWorkspaceId],
  );

  useEffect(() => {
    void loadWorkspaces();
  }, []);

  useEffect(() => {
    if (!selectedWorkspaceId) {
      setConnectors([]);
      return;
    }
    void loadConnectors(selectedWorkspaceId);
  }, [selectedWorkspaceId]);

  useEffect(() => {
    if (selectedWorkspace) {
      setWorkspaceRename(selectedWorkspace.displayName);
    } else {
      setWorkspaceRename("");
    }
  }, [selectedWorkspace]);

  useEffect(() => {
    if (!selectedWorkspaceId) {
      return undefined;
    }

    const timer = window.setInterval(() => {
      void loadConnectors(selectedWorkspaceId);
    }, 15000);

    return () => window.clearInterval(timer);
  }, [selectedWorkspaceId]);

  async function loadWorkspaces(preferredWorkspaceId?: string): Promise<void> {
    try {
      const items = await listWorkspaces();
      setWorkspaces(items);

      const targetId =
        preferredWorkspaceId && items.some((workspace) => workspace.id === preferredWorkspaceId)
          ? preferredWorkspaceId
          : selectedWorkspaceId && items.some((workspace) => workspace.id === selectedWorkspaceId)
            ? selectedWorkspaceId
            : items[0]?.id ?? "";

      setSelectedWorkspaceId(targetId);
      if (!targetId) {
        setConnectors([]);
      }
      setStatus("Ready");
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to load workspaces";
      setError(message);
      setStatus("Disconnected");
    }
  }

  async function loadConnectors(workspaceId: string): Promise<void> {
    try {
      const items = await listConnectors(workspaceId);
      setConnectors(items);
      setConnectorDrafts((current) => {
        const next = { ...current };
        for (const connector of items) {
          if (!next[connector.id]) {
            next[connector.id] = {
              name: connector.name,
              connectorPublicAddr: connector.connectorPublicAddr,
              dataplaneListenAddr: connector.dataplaneListenAddr,
            };
          }
        }
        return next;
      });
      setAgentDraftNames((current) => {
        const next = { ...current };
        for (const connector of items) {
          for (const agent of connector.agents) {
            if (!next[agent.id]) {
              next[agent.id] = agent.name;
            }
          }
        }
        return next;
      });
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to load connectors";
      setError(message);
    }
  }

  async function onCreateWorkspace(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const value = workspaceName.trim();
    if (!value) {
      return;
    }

    try {
      const created = await createWorkspace(value);
      setWorkspaceName("");
      await loadWorkspaces(created.id);
      setStatus(`Workspace "${created.displayName}" created`);
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to create workspace";
      setError(message);
    }
  }

  async function onUpdateWorkspace(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (!selectedWorkspaceId) {
      return;
    }

    const value = workspaceRename.trim();
    if (!value) {
      return;
    }

    try {
      const updated = await updateWorkspace(selectedWorkspaceId, value);
      await loadWorkspaces(updated.id);
      setStatus(`Workspace renamed to "${updated.displayName}"`);
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to update workspace";
      setError(message);
    }
  }

  async function onDeleteWorkspace(): Promise<void> {
    if (!selectedWorkspaceId || !selectedWorkspace) {
      return;
    }

    const shouldDelete = window.confirm(
      `Delete workspace "${selectedWorkspace.displayName}" and all connector/agent records?`,
    );
    if (!shouldDelete) {
      return;
    }

    try {
      await deleteWorkspace(selectedWorkspaceId);
      await loadWorkspaces();
      setStatus(`Workspace "${selectedWorkspace.displayName}" deleted`);
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to delete workspace";
      setError(message);
    }
  }

  async function onCreateConnector(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (!selectedWorkspaceId) {
      setError("Select a workspace first.");
      return;
    }

    try {
      const created = await createConnector({
        workspaceId: selectedWorkspaceId,
        name: connectorName.trim(),
        connectorPublicAddr: connectorPublicAddr.trim(),
        dataplaneListenAddr: dataplaneListenAddr.trim(),
      });

      setConnectorName("");
      setConnectorPublicAddr("");
      setDataplaneListenAddr("0.0.0.0:9443");

      setConnectors((current) => [created, ...current]);
      await loadWorkspaces(selectedWorkspaceId);
      await loadConnectors(selectedWorkspaceId);
      setStatus(`Connector "${created.name}" provisioned`);
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to create connector";
      setError(message);
    }
  }

  async function onUpdateConnector(connector: ConnectorSummary): Promise<void> {
    const draft = connectorDrafts[connector.id] ?? {
      name: connector.name,
      connectorPublicAddr: connector.connectorPublicAddr,
      dataplaneListenAddr: connector.dataplaneListenAddr,
    };

    try {
      const updated = await updateConnector({
        workspaceId: connector.workspaceId,
        connectorId: connector.id,
        name: draft.name.trim(),
        connectorPublicAddr: draft.connectorPublicAddr.trim(),
        dataplaneListenAddr: draft.dataplaneListenAddr.trim(),
      });

      setConnectors((current) =>
        current.map((item) => (item.id === connector.id ? updated : item)),
      );
      await loadConnectors(connector.workspaceId);
      await loadWorkspaces(selectedWorkspaceId || undefined);
      setStatus(`Connector "${updated.name}" updated`);
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to update connector";
      setError(message);
    }
  }

  async function onDeleteConnector(connector: ConnectorSummary): Promise<void> {
    const shouldDelete = window.confirm(
      `Delete connector "${connector.name}" and all its agents?`,
    );
    if (!shouldDelete) {
      return;
    }

    try {
      await deleteConnector(connector.workspaceId, connector.id);
      setConnectors((current) => current.filter((item) => item.id !== connector.id));
      await loadWorkspaces(selectedWorkspaceId || undefined);
      await loadConnectors(connector.workspaceId);
      setStatus(`Connector "${connector.name}" deleted`);
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to delete connector";
      setError(message);
    }
  }

  async function onCreateAgent(
    event: FormEvent<HTMLFormElement>,
    connectorId: string,
    workspaceId: string,
  ): Promise<void> {
    event.preventDefault();
    const value = (agentNames[connectorId] ?? "").trim();
    if (!value) {
      return;
    }

    try {
      const created = await createAgent(workspaceId, connectorId, value);
      setAgentNames((current) => ({ ...current, [connectorId]: "" }));
      setConnectors((current) =>
        current.map((connector) =>
          connector.id === connectorId
            ? {
                ...connector,
                agentCount: connector.agentCount + 1,
                agents: [created, ...connector.agents],
              }
            : connector,
        ),
      );
      await loadWorkspaces(selectedWorkspaceId || undefined);
      await loadConnectors(workspaceId);
      setStatus(`Agent "${created.name}" ready`);
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to create agent";
      setError(message);
    }
  }

  async function onUpdateAgent(connector: ConnectorSummary, agent: AgentSummary): Promise<void> {
    const nextName = (agentDraftNames[agent.id] ?? agent.name).trim();
    if (!nextName) {
      return;
    }

    try {
      const updated = await updateAgent(
        connector.workspaceId,
        connector.id,
        agent.id,
        nextName,
      );
      setConnectors((current) =>
        current.map((item) =>
          item.id === connector.id
            ? {
                ...item,
                agents: item.agents.map((a) => (a.id === agent.id ? updated : a)),
              }
            : item,
        ),
      );
      await loadConnectors(connector.workspaceId);
      setStatus(`Agent "${updated.name}" updated`);
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to update agent";
      setError(message);
    }
  }

  async function onDeleteAgent(connector: ConnectorSummary, agent: AgentSummary): Promise<void> {
    const shouldDelete = window.confirm(`Delete agent "${agent.name}"?`);
    if (!shouldDelete) {
      return;
    }

    try {
      await deleteAgent(connector.workspaceId, connector.id, agent.id);
      setConnectors((current) =>
        current.map((item) =>
          item.id === connector.id
            ? {
                ...item,
                agentCount: Math.max(0, item.agentCount - 1),
                agents: item.agents.filter((a) => a.id !== agent.id),
              }
            : item,
        ),
      );
      await loadWorkspaces(selectedWorkspaceId || undefined);
      await loadConnectors(connector.workspaceId);
      setStatus(`Agent "${agent.name}" deleted`);
      setError("");
    } catch (err) {
      const message = err instanceof Error ? err.message : "failed to delete agent";
      setError(message);
    }
  }

  return (
    <div className="app-shell">
      <aside className="workspace-panel">
        <div className="panel-head">
          <p className="eyebrow">ZTNA</p>
          <h1>Admin Console</h1>
          <p className="subhead">Workspace-centric trust and enrollment management</p>
        </div>

        <div className="workspace-create card">
          <h2>Create Workspace</h2>
          <form onSubmit={onCreateWorkspace}>
            <input
              value={workspaceName}
              onChange={(event) => setWorkspaceName(event.target.value)}
              placeholder="Production LAN"
            />
            <button type="submit">Create</button>
          </form>
        </div>

        <div className="workspace-list card">
          <h2>Workspaces</h2>
          {workspaces.length === 0 ? <p className="hint">No workspaces yet.</p> : null}
          <ul>
            {workspaces.map((workspace) => (
              <li key={workspace.id}>
                <button
                  type="button"
                  className={
                    workspace.id === selectedWorkspaceId ? "workspace-item active" : "workspace-item"
                  }
                  onClick={() => setSelectedWorkspaceId(workspace.id)}
                >
                  <span>{workspace.displayName}</span>
                  <small>
                    {workspace.connectorCount} connectors | {workspace.agentCount} agents
                  </small>
                </button>
              </li>
            ))}
          </ul>
        </div>
      </aside>

      <main className="main-panel">
        <header className="topbar card">
          <div>
            <p className="eyebrow">Workspace</p>
            <h2>{selectedWorkspace?.displayName ?? "Select a workspace"}</h2>
            <code>{selectedWorkspace?.id ?? "-"}</code>
          </div>
          <div className="status-cluster">
            <span className="status-dot" />
            <span>{status}</span>
          </div>
        </header>

        {error ? <div className="error-banner">{error}</div> : null}

        {selectedWorkspace ? (
          <>
            <section className="card workspace-settings-card">
              <h3>Workspace Settings</h3>
              <form className="workspace-settings-form" onSubmit={onUpdateWorkspace}>
                <label>
                  Display Name
                  <input
                    value={workspaceRename}
                    onChange={(event) => setWorkspaceRename(event.target.value)}
                    required
                  />
                </label>
                <div className="row-actions">
                  <button type="submit">Save Workspace</button>
                  <button type="button" className="danger-btn" onClick={() => void onDeleteWorkspace()}>
                    Delete Workspace
                  </button>
                </div>
              </form>
            </section>

            <section className="card provision-card">
              <h3>Create Connector</h3>
              <p>
                This generates a one-time enrollment token and install script URL. Agents created under
                this connector inherit the connector dataplane target.
              </p>
              <form className="connector-form" onSubmit={onCreateConnector}>
                <label>
                  Connector Name
                  <input
                    value={connectorName}
                    onChange={(event) => setConnectorName(event.target.value)}
                    placeholder="hq-gateway-01"
                    required
                  />
                </label>
                <label>
                  Connector Public Address
                  <input
                    value={connectorPublicAddr}
                    onChange={(event) => setConnectorPublicAddr(event.target.value)}
                    placeholder="192.168.1.60:9443"
                    required
                  />
                </label>
                <label>
                  Connector Listen Address
                  <input
                    value={dataplaneListenAddr}
                    onChange={(event) => setDataplaneListenAddr(event.target.value)}
                    placeholder="0.0.0.0:9443"
                    required
                  />
                </label>
                <button type="submit">Create Connector</button>
              </form>
            </section>

            <section className="connectors-grid">
              {connectors.length === 0 ? (
                <div className="card empty-state">
                  <h3>No connectors provisioned</h3>
                  <p>Create your first connector bootstrap profile to start linking agents.</p>
                </div>
              ) : null}

              {connectors.map((connector) => {
                const connectorDraft = connectorDrafts[connector.id] ?? {
                  name: connector.name,
                  connectorPublicAddr: connector.connectorPublicAddr,
                  dataplaneListenAddr: connector.dataplaneListenAddr,
                };

                return (
                  <article className="card connector-card" key={connector.id}>
                    <div className="connector-head">
                      <div>
                        <h3>{connector.name}</h3>
                        <p>
                          {connector.connectorPublicAddr} | {connector.connectorServerName}
                        </p>
                      </div>
                      <div className="status-stack">
                        <StatusBadge label={connector.status.verified ? "Verified" : "Unverified"} tone={connector.status.verified ? "ok" : "warn"} />
                        <StatusBadge label={connector.status.active ? "Active" : "Inactive"} tone={connector.status.active ? "ok" : "neutral"} />
                        <span className="token-badge">expires {new Date(connector.expiresAt).toLocaleString()}</span>
                      </div>
                    </div>

                    <form
                      className="connector-edit-form"
                      onSubmit={(event) => {
                        event.preventDefault();
                        void onUpdateConnector(connector);
                      }}
                    >
                      <label>
                        Connector Name
                        <input
                          value={connectorDraft.name}
                          onChange={(event) =>
                            setConnectorDrafts((current) => ({
                              ...current,
                              [connector.id]: {
                                ...connectorDraft,
                                name: event.target.value,
                              },
                            }))
                          }
                          required
                        />
                      </label>
                      <label>
                        Public Address
                        <input
                          value={connectorDraft.connectorPublicAddr}
                          onChange={(event) =>
                            setConnectorDrafts((current) => ({
                              ...current,
                              [connector.id]: {
                                ...connectorDraft,
                                connectorPublicAddr: event.target.value,
                              },
                            }))
                          }
                          required
                        />
                      </label>
                      <label>
                        Listen Address
                        <input
                          value={connectorDraft.dataplaneListenAddr}
                          onChange={(event) =>
                            setConnectorDrafts((current) => ({
                              ...current,
                              [connector.id]: {
                                ...connectorDraft,
                                dataplaneListenAddr: event.target.value,
                              },
                            }))
                          }
                          required
                        />
                      </label>
                      <div className="row-actions">
                        <button type="submit">Save Connector</button>
                        <button
                          type="button"
                          className="danger-btn"
                          onClick={() => void onDeleteConnector(connector)}
                        >
                          Delete Connector
                        </button>
                      </div>
                    </form>

                    <div className="install-block">
                      <h4>Connector Install</h4>
                      <code>{connector.installCurlCommand}</code>
                      <div className="copy-row">
                        <CopyButton value={connector.installCurlCommand} label="Copy Curl" />
                        <CopyButton value={connector.installScriptUrl} label="Copy Script URL" />
                        <CopyButton value={connector.bootstrapToken} label="Copy Token" />
                      </div>
                    </div>

                    <div className="agent-section">
                      <h4>Agents ({connector.agentCount})</h4>
                      <form
                        className="agent-form"
                        onSubmit={(event) =>
                          void onCreateAgent(event, connector.id, connector.workspaceId)
                        }
                      >
                        <input
                          value={agentNames[connector.id] ?? ""}
                          onChange={(event) =>
                            setAgentNames((current) => ({
                              ...current,
                              [connector.id]: event.target.value,
                            }))
                          }
                          placeholder="engineering-laptop"
                          required
                        />
                        <button type="submit">Create Agent</button>
                      </form>

                      <ul className="agent-list">
                        {connector.agents.map((agent) => {
                          const draftName = agentDraftNames[agent.id] ?? agent.name;
                          return (
                            <li key={agent.id}>
                              <div className="agent-info">
                                <div className="agent-headline">
                                  <strong>{agent.name}</strong>
                                  <div className="status-inline">
                                    <StatusBadge
                                      label={agent.status.verified ? "Verified" : "Unverified"}
                                      tone={agent.status.verified ? "ok" : "warn"}
                                    />
                                    <StatusBadge
                                      label={agent.status.active ? "Active" : "Inactive"}
                                      tone={agent.status.active ? "ok" : "neutral"}
                                    />
                                  </div>
                                </div>
                                <small>
                                  expires {new Date(agent.expiresAt).toLocaleString()} | status {agent.status.deviceStatus}
                                </small>
                                <code>{agent.installCurlCommand}</code>
                                <form
                                  className="agent-edit-form"
                                  onSubmit={(event) => {
                                    event.preventDefault();
                                    void onUpdateAgent(connector, agent);
                                  }}
                                >
                                  <input
                                    value={draftName}
                                    onChange={(event) =>
                                      setAgentDraftNames((current) => ({
                                        ...current,
                                        [agent.id]: event.target.value,
                                      }))
                                    }
                                    required
                                  />
                                  <button type="submit">Save</button>
                                  <button
                                    type="button"
                                    className="danger-btn"
                                    onClick={() => void onDeleteAgent(connector, agent)}
                                  >
                                    Delete
                                  </button>
                                  <CopyButton value={agent.installCurlCommand} label="Copy" />
                                </form>
                              </div>
                            </li>
                          );
                        })}
                      </ul>
                    </div>
                  </article>
                );
              })}
            </section>
          </>
        ) : (
          <section className="card empty-state">
            <h3>No workspace selected</h3>
            <p>Create and select a workspace to provision connectors and agents.</p>
          </section>
        )}
      </main>
    </div>
  );
}

function StatusBadge({ label, tone }: { label: string; tone: "ok" | "warn" | "neutral" }) {
  return <span className={`status-badge status-${tone}`}>{label}</span>;
}
