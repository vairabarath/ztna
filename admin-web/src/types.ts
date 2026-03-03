export interface WorkspaceSummary {
  id: string;
  displayName: string;
  createdAt: string;
  connectorCount: number;
  agentCount: number;
}

export interface DeviceStatusSummary {
  verified: boolean;
  active: boolean;
  deviceStatus: string;
  lastSeenAt: string | null;
}

export interface AgentSummary {
  id: string;
  workspaceId: string;
  connectorId: string;
  managedDeviceId: string;
  name: string;
  createdAt: string;
  expiresAt: string;
  bootstrapToken: string;
  connectorAddr: string;
  connectorServerName: string;
  installScriptUrl: string;
  installCurlCommand: string;
  status: DeviceStatusSummary;
}

export interface ConnectorSummary {
  id: string;
  workspaceId: string;
  managedDeviceId: string;
  name: string;
  createdAt: string;
  expiresAt: string;
  dataplaneListenAddr: string;
  connectorPublicAddr: string;
  connectorServerName: string;
  bootstrapToken: string;
  installScriptUrl: string;
  installCurlCommand: string;
  agentCount: number;
  status: DeviceStatusSummary;
  agents: AgentSummary[];
}
