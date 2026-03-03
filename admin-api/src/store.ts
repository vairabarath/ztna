import fs from "node:fs";
import path from "node:path";

import type { AgentRecord, ConnectorRecord, PersistedState, RemoteNetworkRecord, WorkspaceRecord } from "./types.js";

const EMPTY_STATE: PersistedState = {
  workspaces: [],
  remoteNetworks: [],
  connectors: [],
  agents: [],
};

export class StateStore {
  private readonly filePath: string;

  constructor(filePath: string) {
    this.filePath = filePath;
    this.ensureReady();
  }

  private ensureReady(): void {
    const dir = path.dirname(this.filePath);
    fs.mkdirSync(dir, { recursive: true });
    if (!fs.existsSync(this.filePath)) {
      this.write(EMPTY_STATE);
    }
  }

  private read(): PersistedState {
    const raw = fs.readFileSync(this.filePath, "utf8");
    const parsed = JSON.parse(raw) as Partial<PersistedState>;
    return this.normalizeState(parsed);
  }

  private normalizeState(input: Partial<PersistedState>): PersistedState {
    const workspaces = input.workspaces ?? [];
    const remoteNetworks = input.remoteNetworks ?? [];
    const connectors = (input.connectors ?? []).map((connector) => ({
      ...connector,
      managedDeviceId: normalizeString((connector as Partial<ConnectorRecord>).managedDeviceId) ?? connector.id,
    }));
    const agents = (input.agents ?? []).map((agent) => ({
      ...agent,
      managedDeviceId: normalizeString((agent as Partial<AgentRecord>).managedDeviceId) ?? agent.id,
    }));
    return { workspaces, remoteNetworks, connectors, agents };
  }

  private write(state: PersistedState): void {
    const tempFile = `${this.filePath}.tmp`;
    fs.writeFileSync(tempFile, JSON.stringify(state, null, 2));
    fs.renameSync(tempFile, this.filePath);
  }

  listWorkspaces(): WorkspaceRecord[] {
    return this.read().workspaces;
  }

  getWorkspace(workspaceId: string): WorkspaceRecord | undefined {
    return this.read().workspaces.find((workspace) => workspace.id === workspaceId);
  }

  saveWorkspace(workspace: WorkspaceRecord): WorkspaceRecord {
    const state = this.read();
    state.workspaces = [...state.workspaces.filter((item) => item.id !== workspace.id), workspace];
    this.write(state);
    return workspace;
  }

  updateWorkspace(
    workspaceId: string,
    patch: Partial<Pick<WorkspaceRecord, "displayName">>,
  ): WorkspaceRecord | undefined {
    const state = this.read();
    const index = state.workspaces.findIndex((workspace) => workspace.id === workspaceId);
    if (index < 0) {
      return undefined;
    }

    const current = state.workspaces[index];
    const updated: WorkspaceRecord = {
      ...current,
      displayName: normalizeString(patch.displayName) ?? current.displayName,
    };
    state.workspaces[index] = updated;
    this.write(state);
    return updated;
  }

  deleteWorkspace(workspaceId: string): boolean {
    const state = this.read();
    const hasWorkspace = state.workspaces.some((workspace) => workspace.id === workspaceId);
    if (!hasWorkspace) {
      return false;
    }

    const connectorIds = new Set(
      state.connectors
        .filter((connector) => connector.workspaceId === workspaceId)
        .map((connector) => connector.id),
    );

    state.workspaces = state.workspaces.filter((workspace) => workspace.id !== workspaceId);
    state.remoteNetworks = state.remoteNetworks.filter((rn) => rn.workspaceId !== workspaceId);
    state.connectors = state.connectors.filter((connector) => connector.workspaceId !== workspaceId);
    state.agents = state.agents.filter(
      (agent) => agent.workspaceId !== workspaceId && !connectorIds.has(agent.connectorId),
    );
    this.write(state);
    return true;
  }

  // Remote Networks (Branches)
  listRemoteNetworks(workspaceId: string): RemoteNetworkRecord[] {
    return this.read().remoteNetworks.filter((rn) => rn.workspaceId === workspaceId);
  }

  getRemoteNetwork(remoteNetworkId: string): RemoteNetworkRecord | undefined {
    return this.read().remoteNetworks.find((rn) => rn.id === remoteNetworkId);
  }

  saveRemoteNetwork(remoteNetwork: RemoteNetworkRecord): RemoteNetworkRecord {
    const state = this.read();
    state.remoteNetworks = [...state.remoteNetworks.filter((item) => item.id !== remoteNetwork.id), remoteNetwork];
    this.write(state);
    return remoteNetwork;
  }

  updateRemoteNetwork(
    remoteNetworkId: string,
    patch: Partial<Pick<RemoteNetworkRecord, "name" | "description">>,
  ): RemoteNetworkRecord | undefined {
    const state = this.read();
    const index = state.remoteNetworks.findIndex((rn) => rn.id === remoteNetworkId);
    if (index < 0) {
      return undefined;
    }
    const current = state.remoteNetworks[index];
    const updated: RemoteNetworkRecord = {
      ...current,
      name: normalizeString(patch.name) ?? current.name,
      description: normalizeString(patch.description) ?? current.description,
    };
    state.remoteNetworks[index] = updated;
    this.write(state);
    return updated;
  }

  deleteRemoteNetwork(remoteNetworkId: string): boolean {
    const state = this.read();
    const hasNetwork = state.remoteNetworks.some((rn) => rn.id === remoteNetworkId);
    if (!hasNetwork) {
      return false;
    }

    const connectorIds = new Set(
      state.connectors
        .filter((connector) => connector.remoteNetworkId === remoteNetworkId)
        .map((connector) => connector.id),
    );

    state.remoteNetworks = state.remoteNetworks.filter((rn) => rn.id !== remoteNetworkId);
    state.connectors = state.connectors.filter((connector) => connector.remoteNetworkId !== remoteNetworkId);
    state.agents = state.agents.filter((agent) => !connectorIds.has(agent.connectorId));
    this.write(state);
    return true;
  }

  listConnectors(workspaceId: string): ConnectorRecord[] {
    return this.read().connectors.filter((connector) => connector.workspaceId === workspaceId);
  }

  listConnectorsByRemoteNetwork(remoteNetworkId: string): ConnectorRecord[] {
    return this.read().connectors.filter((connector) => connector.remoteNetworkId === remoteNetworkId);
  }

  getConnector(connectorId: string): ConnectorRecord | undefined {
    return this.read().connectors.find((connector) => connector.id === connectorId);
  }

  saveConnector(connector: ConnectorRecord): ConnectorRecord {
    const state = this.read();
    state.connectors = [...state.connectors.filter((item) => item.id !== connector.id), connector];
    this.write(state);
    return connector;
  }

  updateConnector(
    connectorId: string,
    patch: Partial<Pick<ConnectorRecord, "name" | "connectorPublicAddr" | "dataplaneListenAddr">>,
  ): ConnectorRecord | undefined {
    const state = this.read();
    const index = state.connectors.findIndex((connector) => connector.id === connectorId);
    if (index < 0) {
      return undefined;
    }

    const current = state.connectors[index];
    const connectorPublicAddr = normalizeString(patch.connectorPublicAddr) ?? current.connectorPublicAddr;
    const updated: ConnectorRecord = {
      ...current,
      name: normalizeString(patch.name) ?? current.name,
      connectorPublicAddr,
      dataplaneListenAddr: normalizeString(patch.dataplaneListenAddr) ?? current.dataplaneListenAddr,
    };
    state.connectors[index] = updated;

    // Keep dependent agents aligned with updated connector routing.
    state.agents = state.agents.map((agent) =>
      agent.connectorId === connectorId
        ? {
            ...agent,
            connectorAddr: connectorPublicAddr,
            connectorServerName: updated.connectorServerName,
          }
        : agent,
    );

    this.write(state);
    return updated;
  }

  deleteConnector(connectorId: string): boolean {
    const state = this.read();
    const hasConnector = state.connectors.some((connector) => connector.id === connectorId);
    if (!hasConnector) {
      return false;
    }

    state.connectors = state.connectors.filter((connector) => connector.id !== connectorId);
    state.agents = state.agents.filter((agent) => agent.connectorId !== connectorId);
    this.write(state);
    return true;
  }

  listAgentsByWorkspace(workspaceId: string): AgentRecord[] {
    return this.read().agents.filter((agent) => agent.workspaceId === workspaceId);
  }

  listAgentsByConnector(connectorId: string): AgentRecord[] {
    return this.read().agents.filter((agent) => agent.connectorId === connectorId);
  }

  getAgent(agentId: string): AgentRecord | undefined {
    return this.read().agents.find((agent) => agent.id === agentId);
  }

  saveAgent(agent: AgentRecord): AgentRecord {
    const state = this.read();
    state.agents = [...state.agents.filter((item) => item.id !== agent.id), agent];
    this.write(state);
    return agent;
  }

  updateAgent(
    agentId: string,
    patch: Partial<Pick<AgentRecord, "name">>,
  ): AgentRecord | undefined {
    const state = this.read();
    const index = state.agents.findIndex((agent) => agent.id === agentId);
    if (index < 0) {
      return undefined;
    }

    const current = state.agents[index];
    const updated: AgentRecord = {
      ...current,
      name: normalizeString(patch.name) ?? current.name,
    };
    state.agents[index] = updated;
    this.write(state);
    return updated;
  }

  deleteAgent(agentId: string): boolean {
    const state = this.read();
    const hasAgent = state.agents.some((agent) => agent.id === agentId);
    if (!hasAgent) {
      return false;
    }
    state.agents = state.agents.filter((agent) => agent.id !== agentId);
    this.write(state);
    return true;
  }
}

function normalizeString(value: string | undefined): string | undefined {
  if (!value) {
    return undefined;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}
