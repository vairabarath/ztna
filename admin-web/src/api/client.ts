import type { AgentSummary, ConnectorSummary, RemoteNetworkSummary, WorkspaceSummary } from "../types";

async function parseJSON<T>(response: Response): Promise<T> {
  const data = (await response.json()) as T & { error?: unknown; details?: unknown };
  if (!response.ok) {
    const message = normalizeErrorMessage(data.error) ?? response.statusText;
    throw new Error(message);
  }
  return data;
}

async function post<T>(url: string, body: unknown): Promise<T> {
  const response = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });
  return parseJSON<T>(response);
}

async function patch<T>(url: string, body: unknown): Promise<T> {
  const response = await fetch(url, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });
  return parseJSON<T>(response);
}

async function del(url: string): Promise<void> {
  const response = await fetch(url, { method: "DELETE" });
  if (!response.ok) {
    let errorText = response.statusText;
    try {
      const payload = (await response.json()) as { error?: unknown };
      errorText = normalizeErrorMessage(payload.error) ?? errorText;
    } catch {
      // non-json error body
    }
    throw new Error(errorText);
  }
}

function normalizeErrorMessage(error: unknown): string | null {
  if (typeof error === "string" && error.trim().length > 0) {
    return error;
  }
  if (error && typeof error === "object") {
    const asRecord = error as Record<string, unknown>;
    if (typeof asRecord.message === "string" && asRecord.message.trim().length > 0) {
      return asRecord.message;
    }
    if (Array.isArray(asRecord.formErrors) && asRecord.formErrors.length > 0) {
      const first = asRecord.formErrors.find(
        (item) => typeof item === "string" && item.trim().length > 0,
      );
      if (typeof first === "string") {
        return first;
      }
    }
  }
  return null;
}

export async function listWorkspaces(): Promise<WorkspaceSummary[]> {
  const response = await fetch("/api/workspaces");
  const payload = await parseJSON<{ items: WorkspaceSummary[] }>(response);
  return payload.items;
}

export async function createWorkspace(displayName: string): Promise<WorkspaceSummary> {
  const payload = await post<{ item: WorkspaceSummary }>("/api/workspaces", { displayName });
  return payload.item;
}

export async function updateWorkspace(
  workspaceId: string,
  displayName: string,
): Promise<WorkspaceSummary> {
  const payload = await patch<{ item: WorkspaceSummary }>(`/api/workspaces/${workspaceId}`, {
    displayName,
  });
  return payload.item;
}

export async function deleteWorkspace(workspaceId: string): Promise<void> {
  await del(`/api/workspaces/${workspaceId}`);
}

// Remote Networks API
export async function listRemoteNetworks(workspaceId: string): Promise<RemoteNetworkSummary[]> {
  const response = await fetch(`/api/workspaces/${workspaceId}/networks`);
  const payload = await parseJSON<{ items: RemoteNetworkSummary[] }>(response);
  return payload.items;
}

export interface CreateRemoteNetworkInput {
  workspaceId: string;
  name: string;
  description?: string;
}

export async function createRemoteNetwork(input: CreateRemoteNetworkInput): Promise<RemoteNetworkSummary> {
  const payload = await post<{ item: RemoteNetworkSummary }>(
    `/api/workspaces/${input.workspaceId}/networks`,
    { name: input.name, description: input.description },
  );
  return payload.item;
}

export async function updateRemoteNetwork(
  workspaceId: string,
  networkId: string,
  name: string,
  description?: string,
): Promise<RemoteNetworkSummary> {
  const payload = await patch<{ item: RemoteNetworkSummary }>(
    `/api/workspaces/${workspaceId}/networks/${networkId}`,
    { name, description },
  );
  return payload.item;
}

export async function deleteRemoteNetwork(workspaceId: string, networkId: string): Promise<void> {
  await del(`/api/workspaces/${workspaceId}/networks/${networkId}`);
}

// Connectors API
export async function listConnectors(workspaceId: string): Promise<ConnectorSummary[]> {
  const response = await fetch(`/api/workspaces/${workspaceId}/connectors`);
  const payload = await parseJSON<{ items: ConnectorSummary[] }>(response);
  return payload.items;
}

export interface CreateConnectorInput {
  workspaceId: string;
  remoteNetworkId: string;
  name: string;
  connectorPublicAddr: string;
  dataplaneListenAddr: string;
}

export async function createConnector(input: CreateConnectorInput): Promise<ConnectorSummary> {
  const payload = await post<{ item: ConnectorSummary }>(
    `/api/workspaces/${input.workspaceId}/connectors`,
    {
      remoteNetworkId: input.remoteNetworkId,
      name: input.name,
      connectorPublicAddr: input.connectorPublicAddr,
      dataplaneListenAddr: input.dataplaneListenAddr,
    },
  );
  return payload.item;
}

export interface UpdateConnectorInput {
  workspaceId: string;
  connectorId: string;
  name?: string;
  connectorPublicAddr?: string;
  dataplaneListenAddr?: string;
}

export async function updateConnector(input: UpdateConnectorInput): Promise<ConnectorSummary> {
  const payload = await patch<{ item: ConnectorSummary }>(
    `/api/workspaces/${input.workspaceId}/connectors/${input.connectorId}`,
    {
      name: input.name,
      connectorPublicAddr: input.connectorPublicAddr,
      dataplaneListenAddr: input.dataplaneListenAddr,
    },
  );
  return payload.item;
}

export async function deleteConnector(workspaceId: string, connectorId: string): Promise<void> {
  await del(`/api/workspaces/${workspaceId}/connectors/${connectorId}`);
}

export async function createAgent(
  workspaceId: string,
  connectorId: string,
  name: string,
): Promise<AgentSummary> {
  const payload = await post<{ item: AgentSummary }>(
    `/api/workspaces/${workspaceId}/connectors/${connectorId}/agents`,
    { name },
  );
  return payload.item;
}

export async function updateAgent(
  workspaceId: string,
  connectorId: string,
  agentId: string,
  name: string,
): Promise<AgentSummary> {
  const payload = await patch<{ item: AgentSummary }>(
    `/api/workspaces/${workspaceId}/connectors/${connectorId}/agents/${agentId}`,
    { name },
  );
  return payload.item;
}

export async function deleteAgent(
  workspaceId: string,
  connectorId: string,
  agentId: string,
): Promise<void> {
  await del(`/api/workspaces/${workspaceId}/connectors/${connectorId}/agents/${agentId}`);
}
