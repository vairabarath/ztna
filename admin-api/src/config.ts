import fs from "node:fs";
import path from "node:path";

function env(name: string, fallback: string): string {
  const value = process.env[name];
  if (!value || !value.trim()) {
    return fallback;
  }
  return value.trim();
}

const rootDir = path.resolve(path.dirname(new URL(import.meta.url).pathname), "../../");
const defaultCACertPath = path.resolve(rootDir, "deploy/tls/controller.crt");

export interface AppConfig {
  port: number;
  corsOrigin: string;
  controllerTarget: string;
  controllerTLS: boolean;
  controllerTLSServerName: string;
  controllerCACertPEM: string;
  adminToken: string;
  tokenTTLHours: number;
  publicBaseURL: string;
  stateFilePath: string;
  deviceActiveWindowSecs: number;
  githubRepoOwner: string;
  githubRepoName: string;
}

export function loadConfig(): AppConfig {
  const controllerTargetRaw = env("ADMIN_CONTROLLER_ADDR", "127.0.0.1:8443");
  const targetHasScheme =
    controllerTargetRaw.startsWith("https://") || controllerTargetRaw.startsWith("http://");
  const controllerTLS = targetHasScheme
    ? controllerTargetRaw.startsWith("https://")
    : true;
  const controllerTarget = controllerTargetRaw.replace(/^https?:\/\//, "");
  const controllerHost = controllerTarget.split(":")[0];
  const isIPv4 = /^(?:\d{1,3}\.){3}\d{1,3}$/.test(controllerHost);
  const tlsServerName = env(
    "ADMIN_CONTROLLER_TLS_SERVER_NAME",
    isIPv4 ? "localhost" : controllerHost,
  );

  const caPath = env("ADMIN_CONTROLLER_CA_CERT_FILE", defaultCACertPath);
  const stateFilePath = env(
    "ADMIN_STATE_FILE",
    path.resolve(rootDir, "admin-api/data/state.json"),
  );

  const caCert = fs.readFileSync(caPath, "utf8");

  return {
    port: Number(env("ADMIN_API_PORT", "8787")),
    corsOrigin: env("ADMIN_CORS_ORIGIN", "*"),
    controllerTarget,
    controllerTLS,
    controllerTLSServerName: tlsServerName,
    controllerCACertPEM: caCert,
    adminToken: env("ADMIN_TOKEN", "dev-admin-token"),
    tokenTTLHours: Number(env("ADMIN_TOKEN_TTL_HOURS", "24")),
    publicBaseURL: env("ADMIN_PUBLIC_BASE_URL", "http://localhost:8787"),
    stateFilePath,
    deviceActiveWindowSecs: Number(env("ADMIN_DEVICE_ACTIVE_WINDOW_SECS", "45")),
    githubRepoOwner: env("ZTNA_REPO_OWNER", "vairabarath"),
    githubRepoName: env("ZTNA_REPO_NAME", "ztna"),
  };
}
