export type TokenKind = "connector" | "agent";

export interface WorkspaceRecord {
  id: string;
  displayName: string;
  caCertPem: string;
  createdAt: string;
}

export interface ConnectorRecord {
  id: string;
  workspaceId: string;
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
  connectors: ConnectorRecord[];
  agents: AgentRecord[];
}
