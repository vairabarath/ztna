export type TokenKind = "connector" | "agent";

// Company boundary
export interface WorkspaceRecord {
  id: string;
  displayName: string;
  caCertPem: string;
  createdAt: string;
}

// Remote Network (Branch/Location) within a workspace
export interface RemoteNetworkRecord {
  id: string;
  workspaceId: string;
  name: string;
  description: string;
  createdAt: string;
}

// Connector within a remote network
export interface ConnectorRecord {
  id: string;
  workspaceId: string;
  remoteNetworkId: string;  // NEW: Links to remote network
  managedDeviceId: string;
  name: string;
  bootstrapToken: string;
  createdAt: string;
  expiresAt: string;
  controllerAddr: string;
  controllerCaCertPem: string;
  dataplaneListenAddr: string;
  connectorPublicAddr: string;
  connectorServerName: string;
}

export interface AgentRecord {
  id: string;
  workspaceId: string;
  connectorId: string;
  managedDeviceId: string;
  name: string;
  bootstrapToken: string;
  createdAt: string;
  expiresAt: string;
  controllerAddr: string;
  controllerCaCertPem: string;
  connectorAddr: string;
  connectorServerName: string;
}

export interface PersistedState {
  workspaces: WorkspaceRecord[];
  remoteNetworks: RemoteNetworkRecord[];  // NEW
  connectors: ConnectorRecord[];
  agents: AgentRecord[];
}
