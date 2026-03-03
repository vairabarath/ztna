import express from "express";
import cors from "cors";
import grpc from "@grpc/grpc-js";
import { randomUUID } from "node:crypto";
import { z } from "zod";

import { loadConfig } from "./config.js";
import { ControlPlaneClient, type DeviceStatus } from "./grpc.js";
import {
  buildAgentInstallCommand,
  buildAgentInstallScript,
  buildConnectorInstallCommand,
  buildConnectorInstallScript,
} from "./install.js";
import { StateStore } from "./store.js";
import type { AgentRecord, ConnectorRecord, WorkspaceRecord } from "./types.js";

const cfg = loadConfig();
const app = express();
const store = new StateStore(cfg.stateFilePath);
const controlPlane = new ControlPlaneClient(cfg);

app.use(cors({ origin: cfg.corsOrigin }));
app.use(express.json({ limit: "1mb" }));

const createWorkspaceSchema = z.object({
  displayName: z.string().trim().min(2).max(80),
});

const updateWorkspaceSchema = z.object({
  displayName: z.string().trim().min(2).max(80),
});

const connectorPublicAddrSchema = z
  .string()
  .trim()
  .min(3)
  .max(200)
  .regex(/:\d+$/, "connectorPublicAddr must include port, e.g. 192.168.1.60:9443");

const createConnectorSchema = z.object({
  name: z.string().trim().min(2).max(80),
  connectorPublicAddr: connectorPublicAddrSchema,
  dataplaneListenAddr: z.string().trim().min(3).max(200).default("0.0.0.0:9443"),
});

const updateConnectorSchema = z
  .object({
    name: z.string().trim().min(2).max(80).optional(),
    connectorPublicAddr: connectorPublicAddrSchema.optional(),
    dataplaneListenAddr: z.string().trim().min(3).max(200).optional(),
  })
  .refine(
    (value) =>
      value.name !== undefined ||
      value.connectorPublicAddr !== undefined ||
      value.dataplaneListenAddr !== undefined,
    {
      message: "at least one connector field must be provided",
    },
  );

const createAgentSchema = z.object({
  name: z.string().trim().min(2).max(80),
});

const updateAgentSchema = z
  .object({
    name: z.string().trim().min(2).max(80).optional(),
  })
  .refine((value) => value.name !== undefined, {
    message: "at least one agent field must be provided",
  });

app.get("/health", (_req, res) => {
  res.json({
    status: "ok",
    service: "ztna-admin-api",
    controllerTarget: cfg.controllerTarget,
    controllerTLS: cfg.controllerTLS,
  });
});

app.get("/api/workspaces", (_req, res) => {
  const payload = store.listWorkspaces().map((workspace) => toWorkspaceDTO(workspace));
  res.json({ items: payload });
});

app.post("/api/workspaces", async (req, res) => {
  const parsed = createWorkspaceSchema.safeParse(req.body);
  if (!parsed.success) {
    res.status(400).json(validationError(parsed.error));
    return;
  }

  try {
    const response = await controlPlane.createWorkspace(parsed.data.displayName);
    const workspace: WorkspaceRecord = {
      id: response.workspaceId,
      displayName: parsed.data.displayName,
      caCertPem: response.caCertPem,
      createdAt: response.createdAt,
    };

    store.saveWorkspace(workspace);

    res.status(201).json({ item: toWorkspaceDTO(workspace) });
  } catch (error) {
    handleError(error, res);
  }
});

app.patch("/api/workspaces/:workspaceId", (req, res) => {
  const parsed = updateWorkspaceSchema.safeParse(req.body);
  if (!parsed.success) {
    res.status(400).json(validationError(parsed.error));
    return;
  }

  const updated = store.updateWorkspace(req.params.workspaceId, {
    displayName: parsed.data.displayName,
  });
  if (!updated) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }

  res.json({ item: toWorkspaceDTO(updated) });
});

app.delete("/api/workspaces/:workspaceId", (req, res) => {
  const deleted = store.deleteWorkspace(req.params.workspaceId);
  if (!deleted) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }
  res.status(204).send();
});

app.get("/api/workspaces/:workspaceId/connectors", async (req, res) => {
  const workspaceId = req.params.workspaceId;
  const workspace = store.getWorkspace(workspaceId);
  if (!workspace) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }

  try {
    const deviceStatusById = await loadDeviceStatusMap(workspaceId);
    const connectors = store
      .listConnectors(workspaceId)
      .map((connector) => toConnectorDTO(connector, deviceStatusById));

    res.json({ items: connectors });
  } catch (error) {
    handleError(error, res);
  }
});

app.post("/api/workspaces/:workspaceId/connectors", async (req, res) => {
  const workspaceId = req.params.workspaceId;
  const workspace = store.getWorkspace(workspaceId);
  if (!workspace) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }

  const parsed = createConnectorSchema.safeParse(req.body);
  if (!parsed.success) {
    res.status(400).json(validationError(parsed.error));
    return;
  }

  try {
    const expiresAt = new Date(Date.now() + cfg.tokenTTLHours * 60 * 60 * 1000);
    const tokenResult = await controlPlane.createConnectorToken(workspaceId, expiresAt);
    const connectorId = randomUUID();

    const connector: ConnectorRecord = {
      id: connectorId,
      workspaceId,
      managedDeviceId: connectorId,
      name: parsed.data.name,
      bootstrapToken: tokenResult.token,
      createdAt: new Date().toISOString(),
      expiresAt: tokenResult.expiresAt,
      controllerAddr: fullControllerAddr(),
      controllerCaCertPem: cfg.controllerCACertPEM,
      dataplaneListenAddr: parsed.data.dataplaneListenAddr,
      connectorPublicAddr: parsed.data.connectorPublicAddr,
      connectorServerName: `connector.${workspaceId}`,
    };

    store.saveConnector(connector);

    res.status(201).json({ item: toConnectorDTO(connector) });
  } catch (error) {
    handleError(error, res);
  }
});

app.patch("/api/workspaces/:workspaceId/connectors/:connectorId", (req, res) => {
  const { workspaceId, connectorId } = req.params;
  const workspace = store.getWorkspace(workspaceId);
  if (!workspace) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }

  const connector = store.getConnector(connectorId);
  if (!connector || connector.workspaceId !== workspaceId) {
    res.status(404).json({ error: "connector not found" });
    return;
  }

  const parsed = updateConnectorSchema.safeParse(req.body);
  if (!parsed.success) {
    res.status(400).json(validationError(parsed.error));
    return;
  }

  const updated = store.updateConnector(connectorId, parsed.data);
  if (!updated) {
    res.status(404).json({ error: "connector not found" });
    return;
  }

  res.json({ item: toConnectorDTO(updated) });
});

app.delete("/api/workspaces/:workspaceId/connectors/:connectorId", (req, res) => {
  const { workspaceId, connectorId } = req.params;
  const workspace = store.getWorkspace(workspaceId);
  if (!workspace) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }

  const connector = store.getConnector(connectorId);
  if (!connector || connector.workspaceId !== workspaceId) {
    res.status(404).json({ error: "connector not found" });
    return;
  }

  const deleted = store.deleteConnector(connectorId);
  if (!deleted) {
    res.status(404).json({ error: "connector not found" });
    return;
  }

  res.status(204).send();
});

app.get("/api/workspaces/:workspaceId/connectors/:connectorId/agents", async (req, res) => {
  const { workspaceId, connectorId } = req.params;
  const workspace = store.getWorkspace(workspaceId);
  if (!workspace) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }

  const connector = store.getConnector(connectorId);
  if (!connector || connector.workspaceId !== workspaceId) {
    res.status(404).json({ error: "connector not found" });
    return;
  }

  try {
    const deviceStatusById = await loadDeviceStatusMap(workspaceId);
    const agents = store
      .listAgentsByConnector(connectorId)
      .map((agent) => toAgentDTO(agent, deviceStatusById));
    res.json({ items: agents });
  } catch (error) {
    handleError(error, res);
  }
});

app.post("/api/workspaces/:workspaceId/connectors/:connectorId/agents", async (req, res) => {
  const { workspaceId, connectorId } = req.params;
  const workspace = store.getWorkspace(workspaceId);
  if (!workspace) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }

  const connector = store.getConnector(connectorId);
  if (!connector || connector.workspaceId !== workspaceId) {
    res.status(404).json({ error: "connector not found" });
    return;
  }

  const parsed = createAgentSchema.safeParse(req.body);
  if (!parsed.success) {
    res.status(400).json(validationError(parsed.error));
    return;
  }

  try {
    const expiresAt = new Date(Date.now() + cfg.tokenTTLHours * 60 * 60 * 1000);
    const tokenResult = await controlPlane.createAgentToken(workspaceId, expiresAt);
    const agentId = randomUUID();

    const agent: AgentRecord = {
      id: agentId,
      workspaceId,
      connectorId,
      managedDeviceId: agentId,
      name: parsed.data.name,
      bootstrapToken: tokenResult.token,
      createdAt: new Date().toISOString(),
      expiresAt: tokenResult.expiresAt,
      controllerAddr: fullControllerAddr(),
      controllerCaCertPem: cfg.controllerCACertPEM,
      connectorAddr: connector.connectorPublicAddr,
      connectorServerName: connector.connectorServerName,
    };

    store.saveAgent(agent);

    res.status(201).json({ item: toAgentDTO(agent) });
  } catch (error) {
    handleError(error, res);
  }
});

app.patch("/api/workspaces/:workspaceId/connectors/:connectorId/agents/:agentId", (req, res) => {
  const { workspaceId, connectorId, agentId } = req.params;
  const workspace = store.getWorkspace(workspaceId);
  if (!workspace) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }

  const connector = store.getConnector(connectorId);
  if (!connector || connector.workspaceId !== workspaceId) {
    res.status(404).json({ error: "connector not found" });
    return;
  }

  const agent = store.getAgent(agentId);
  if (!agent || agent.workspaceId !== workspaceId || agent.connectorId !== connectorId) {
    res.status(404).json({ error: "agent not found" });
    return;
  }

  const parsed = updateAgentSchema.safeParse(req.body);
  if (!parsed.success) {
    res.status(400).json(validationError(parsed.error));
    return;
  }

  const updated = store.updateAgent(agentId, parsed.data);
  if (!updated) {
    res.status(404).json({ error: "agent not found" });
    return;
  }

  res.json({ item: toAgentDTO(updated) });
});

app.delete("/api/workspaces/:workspaceId/connectors/:connectorId/agents/:agentId", (req, res) => {
  const { workspaceId, connectorId, agentId } = req.params;
  const workspace = store.getWorkspace(workspaceId);
  if (!workspace) {
    res.status(404).json({ error: "workspace not found" });
    return;
  }

  const connector = store.getConnector(connectorId);
  if (!connector || connector.workspaceId !== workspaceId) {
    res.status(404).json({ error: "connector not found" });
    return;
  }

  const agent = store.getAgent(agentId);
  if (!agent || agent.workspaceId !== workspaceId || agent.connectorId !== connectorId) {
    res.status(404).json({ error: "agent not found" });
    return;
  }

  const deleted = store.deleteAgent(agentId);
  if (!deleted) {
    res.status(404).json({ error: "agent not found" });
    return;
  }

  res.status(204).send();
});

app.get("/install/connector/:connectorId.sh", (req, res) => {
  const connector = store.getConnector(req.params.connectorId);
  if (!connector) {
    res.status(404).type("text/plain").send("connector install profile not found\n");
    return;
  }

  if (new Date(connector.expiresAt).getTime() < Date.now()) {
    res.status(410).type("text/plain").send("connector bootstrap token expired\n");
    return;
  }

  res
    .setHeader("Cache-Control", "no-store")
    .type("text/x-shellscript")
    .send(
      buildConnectorInstallScript(
        connector,
        cfg.controllerCACertPEM,
        fullControllerAddr(),
      ),
    );
});

app.get("/install/agent/:agentId.sh", (req, res) => {
  const agent = store.getAgent(req.params.agentId);
  if (!agent) {
    res.status(404).type("text/plain").send("agent install profile not found\n");
    return;
  }

  if (new Date(agent.expiresAt).getTime() < Date.now()) {
    res.status(410).type("text/plain").send("agent bootstrap token expired\n");
    return;
  }

  res
    .setHeader("Cache-Control", "no-store")
    .type("text/x-shellscript")
    .send(
      buildAgentInstallScript(
        agent,
        cfg.controllerCACertPEM,
        fullControllerAddr(),
      ),
    );
});

app.listen(cfg.port, () => {
  // eslint-disable-next-line no-console
  console.log(`ztna-admin-api listening on http://localhost:${cfg.port}`);
});

function fullControllerAddr(): string {
  return `${cfg.controllerTLS ? "https" : "http"}://${cfg.controllerTarget}`;
}

function toWorkspaceDTO(workspace: WorkspaceRecord) {
  return {
    id: workspace.id,
    displayName: workspace.displayName,
    createdAt: workspace.createdAt,
    connectorCount: store.listConnectors(workspace.id).length,
    agentCount: store.listAgentsByWorkspace(workspace.id).length,
  };
}

function toConnectorDTO(
  connector: ConnectorRecord,
  deviceStatusById: Map<string, DeviceStatus> = new Map(),
) {
  const controllerAddr = fullControllerAddr();
  const legacyScriptUrl = `${cfg.publicBaseURL}/install/connector/${connector.id}.sh`;

  // Twingate-style install command using the main install.sh script
  const twingateStyleCommand = buildConnectorInstallCommand(connector, controllerAddr);

  return {
    id: connector.id,
    workspaceId: connector.workspaceId,
    managedDeviceId: connector.managedDeviceId,
    name: connector.name,
    createdAt: connector.createdAt,
    expiresAt: connector.expiresAt,
    dataplaneListenAddr: connector.dataplaneListenAddr,
    connectorPublicAddr: connector.connectorPublicAddr,
    connectorServerName: connector.connectorServerName,
    bootstrapToken: connector.bootstrapToken,
    // Legacy install script URL (self-hosted)
    installScriptUrl: legacyScriptUrl,
    // Twingate-style curl command (recommended)
    installCurlCommand: twingateStyleCommand,
    agentCount: store.listAgentsByConnector(connector.id).length,
    status: resolveDeviceStatus(connector.managedDeviceId, deviceStatusById),
    agents: store
      .listAgentsByConnector(connector.id)
      .map((agent) => toAgentDTO(agent, deviceStatusById)),
  };
}

function toAgentDTO(agent: AgentRecord, deviceStatusById: Map<string, DeviceStatus> = new Map()) {
  const controllerAddr = fullControllerAddr();
  const legacyScriptUrl = `${cfg.publicBaseURL}/install/agent/${agent.id}.sh`;

  // Twingate-style install command using the main install.sh script
  const twingateStyleCommand = buildAgentInstallCommand(agent, controllerAddr);

  return {
    id: agent.id,
    workspaceId: agent.workspaceId,
    connectorId: agent.connectorId,
    managedDeviceId: agent.managedDeviceId,
    name: agent.name,
    createdAt: agent.createdAt,
    expiresAt: agent.expiresAt,
    bootstrapToken: agent.bootstrapToken,
    connectorAddr: agent.connectorAddr,
    connectorServerName: agent.connectorServerName,
    // Legacy install script URL (self-hosted)
    installScriptUrl: legacyScriptUrl,
    // Twingate-style curl command (recommended)
    installCurlCommand: twingateStyleCommand,
    status: resolveDeviceStatus(agent.managedDeviceId, deviceStatusById),
  };
}

function resolveDeviceStatus(deviceId: string, deviceStatusById: Map<string, DeviceStatus>) {
  const found = deviceStatusById.get(deviceId);
  if (!found) {
    return {
      verified: false,
      active: false,
      deviceStatus: "pending",
      lastSeenAt: null,
    };
  }

  const normalized = normalizeStatus(found.status);
  const lastSeenAt = found.lastSeenAt || null;
  const activeByRecency = isRecent(lastSeenAt, cfg.deviceActiveWindowSecs);
  const active = normalized === "active" && activeByRecency;
  return {
    verified: true,
    active,
    deviceStatus: normalized === "active" && !active ? "stale" : normalized,
    lastSeenAt,
  };
}

async function loadDeviceStatusMap(workspaceId: string): Promise<Map<string, DeviceStatus>> {
  try {
    const devices = await controlPlane.listDevices(workspaceId);
    return new Map(devices.map((item) => [item.deviceId, item]));
  } catch (error) {
    // Backward-compatible fallback while old controller versions are still running.
    if (isGRPCError(error) && error.code === grpc.status.UNIMPLEMENTED) {
      return new Map();
    }
    throw error;
  }
}

function normalizeStatus(value: string): string {
  const normalized = value.trim().toLowerCase();
  return normalized.length > 0 ? normalized : "unknown";
}

function validationError(err: z.ZodError): { error: string; details: unknown } {
  const firstMessage = err.issues.find((issue) => issue.message.trim().length > 0)?.message;
  return {
    error: firstMessage ?? "invalid request payload",
    details: err.flatten(),
  };
}

function isRecent(iso: string | null, windowSeconds: number): boolean {
  if (!iso) {
    return false;
  }
  const millis = Date.parse(iso);
  if (Number.isNaN(millis)) {
    return false;
  }
  const ageMillis = Date.now() - millis;
  return ageMillis >= 0 && ageMillis <= windowSeconds * 1000;
}

function handleError(error: unknown, res: express.Response): void {
  if (isGRPCError(error)) {
    const status = grpcToHttp(error.code);
    res.status(status).json({
      error: error.details || error.message,
      grpcCode: error.code,
    });
    return;
  }

  const message = error instanceof Error ? error.message : "unexpected error";
  res.status(500).json({ error: message });
}

function isGRPCError(error: unknown): error is grpc.ServiceError {
  return error !== null && typeof error === "object" && "code" in error;
}

function grpcToHttp(code: number): number {
  switch (code) {
    case grpc.status.INVALID_ARGUMENT:
      return 400;
    case grpc.status.UNAUTHENTICATED:
      return 401;
    case grpc.status.PERMISSION_DENIED:
      return 403;
    case grpc.status.NOT_FOUND:
      return 404;
    case grpc.status.ALREADY_EXISTS:
      return 409;
    case grpc.status.FAILED_PRECONDITION:
      return 412;
    default:
      return 502;
  }
}
