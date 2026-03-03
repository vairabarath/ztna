import path from "node:path";

import grpc from "@grpc/grpc-js";
import protoLoader from "@grpc/proto-loader";

import type { AppConfig } from "./config.js";
import { protoTimestampToISO, toProtoTimestamp } from "./time.js";

const enum ConnectorTokenType {
  ENROLL_TOKEN_TYPE_CONNECTOR = 1,
  ENROLL_TOKEN_TYPE_AGENT = 2,
}

type WorkspaceGRPCClient = grpc.Client & {
  CreateWorkspace: (
    request: { display_name: string } | { displayName: string },
    metadata: grpc.Metadata,
    callback: (err: grpc.ServiceError | null, response?: unknown) => void,
  ) => void;
  CreateEnrollToken: (
    request:
      | { workspace_id: string; type: number; expires_at: { seconds: number; nanos: number } }
      | { workspaceId: string; type: number; expiresAt: { seconds: number; nanos: number } },
    metadata: grpc.Metadata,
    callback: (err: grpc.ServiceError | null, response?: unknown) => void,
  ) => void;
  GetWorkspaceCA: (
    request: { workspace_id: string } | { workspaceId: string },
    callback: (err: grpc.ServiceError | null, response?: unknown) => void,
  ) => void;
};

type DeviceGRPCClient = grpc.Client & {
  ListDevices: (
    request: { workspace_id: string } | { workspaceId: string },
    metadata: grpc.Metadata,
    callback: (err: grpc.ServiceError | null, response?: unknown) => void,
  ) => void;
};

interface CreateWorkspaceResult {
  workspaceId: string;
  caCertPem: string;
  createdAt: string;
}

interface CreateEnrollTokenResult {
  tokenId: string;
  token: string;
  expiresAt: string;
}

export interface DeviceStatus {
  workspaceId: string;
  deviceId: string;
  certFingerprint: string;
  status: string;
  lastSeenAt: string;
}

export class ControlPlaneClient {
  private readonly workspaceClient: WorkspaceGRPCClient;
  private readonly deviceClient: DeviceGRPCClient;
  private readonly adminToken: string;

  constructor(config: AppConfig) {
    this.adminToken = config.adminToken;

    const protoPath = path.resolve(
      path.dirname(new URL(import.meta.url).pathname),
      "../../proto/ztna/controlplane/v1/controlplane.proto",
    );

    const def = protoLoader.loadSync(protoPath, {
      keepCase: false,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
    });

    const pkgDef = grpc.loadPackageDefinition(def) as grpc.GrpcObject;
    const ztnaPkg = pkgDef.ztna as grpc.GrpcObject;
    const controlplanePkg = (ztnaPkg.controlplane as grpc.GrpcObject).v1 as grpc.GrpcObject;
    const WorkspaceService = controlplanePkg.WorkspaceService as grpc.ServiceClientConstructor;
    const DeviceService = controlplanePkg.DeviceService as grpc.ServiceClientConstructor;

    const creds = config.controllerTLS
      ? grpc.credentials.createSsl(Buffer.from(config.controllerCACertPEM, "utf8"))
      : grpc.credentials.createInsecure();
    const options = config.controllerTLS
      ? {
          "grpc.ssl_target_name_override": config.controllerTLSServerName,
          "grpc.default_authority": config.controllerTLSServerName,
        }
      : undefined;

    this.workspaceClient = new WorkspaceService(
      config.controllerTarget,
      creds,
      options,
    ) as unknown as WorkspaceGRPCClient;
    this.deviceClient = new DeviceService(
      config.controllerTarget,
      creds,
      options,
    ) as unknown as DeviceGRPCClient;
  }

  async createWorkspace(displayName: string): Promise<CreateWorkspaceResult> {
    const metadata = this.adminMetadata();
    const response = await unary<unknown>((callback) =>
      this.workspaceClient.CreateWorkspace({ displayName }, metadata, callback),
    );
    const payload = response as {
      workspaceId?: string;
      caCertPem?: string;
      createdAt?: unknown;
    };

    return {
      workspaceId: payload.workspaceId ?? "",
      caCertPem: payload.caCertPem ?? "",
      createdAt: protoTimestampToISO(payload.createdAt),
    };
  }

  async createConnectorToken(workspaceId: string, expiresAt: Date): Promise<CreateEnrollTokenResult> {
    return this.createEnrollToken(
      workspaceId,
      ConnectorTokenType.ENROLL_TOKEN_TYPE_CONNECTOR,
      expiresAt,
    );
  }

  async createAgentToken(workspaceId: string, expiresAt: Date): Promise<CreateEnrollTokenResult> {
    return this.createEnrollToken(workspaceId, ConnectorTokenType.ENROLL_TOKEN_TYPE_AGENT, expiresAt);
  }

  async getWorkspaceCA(workspaceId: string): Promise<{ workspaceId: string; caCertPem: string }> {
    const response = await unary<unknown>((callback) =>
      this.workspaceClient.GetWorkspaceCA({ workspaceId }, callback),
    );
    const payload = response as { workspaceId?: string; caCertPem?: string };
    return {
      workspaceId: payload.workspaceId ?? workspaceId,
      caCertPem: payload.caCertPem ?? "",
    };
  }

  async listDevices(workspaceId: string): Promise<DeviceStatus[]> {
    const metadata = this.adminMetadata();
    const response = await unary<unknown>((callback) =>
      this.deviceClient.ListDevices({ workspaceId }, metadata, callback),
    );
    const payload = response as {
      items?: Array<{
        workspaceId?: string;
        deviceId?: string;
        certFingerprint?: string;
        status?: string;
        lastSeenAt?: unknown;
      }>;
    };

    const items = payload.items ?? [];
    return items.map((item) => ({
      workspaceId: item.workspaceId ?? workspaceId,
      deviceId: item.deviceId ?? "",
      certFingerprint: item.certFingerprint ?? "",
      status: (item.status ?? "").toLowerCase(),
      lastSeenAt: protoTimestampToISO(item.lastSeenAt),
    }));
  }

  private async createEnrollToken(
    workspaceId: string,
    tokenType: number,
    expiresAt: Date,
  ): Promise<CreateEnrollTokenResult> {
    const metadata = this.adminMetadata();
    const response = await unary<unknown>((callback) =>
      this.workspaceClient.CreateEnrollToken(
        {
          workspaceId,
          type: tokenType,
          expiresAt: toProtoTimestamp(expiresAt),
        },
        metadata,
        callback,
      ),
    );

    const payload = response as {
      tokenId?: string;
      token?: string;
      expiresAt?: unknown;
    };

    return {
      tokenId: payload.tokenId ?? "",
      token: payload.token ?? "",
      expiresAt: protoTimestampToISO(payload.expiresAt),
    };
  }

  private adminMetadata(): grpc.Metadata {
    const metadata = new grpc.Metadata();
    metadata.set("x-admin-token", this.adminToken);
    return metadata;
  }
}

function unary<T>(
  invoke: (callback: (err: grpc.ServiceError | null, response?: T) => void) => void,
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    invoke((err, response) => {
      if (err) {
        reject(err);
        return;
      }
      if (!response) {
        reject(new Error("empty gRPC response"));
        return;
      }
      resolve(response);
    });
  });
}
