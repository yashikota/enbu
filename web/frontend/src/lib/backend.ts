import {
  api,
  type AuthStatus,
  type Environment,
  type GUIRepoStatus,
  type InitResult,
  type SecretsResponse,
} from "./api";
import { mockBackend } from "./mock-backend";

const isMock =
  (typeof window !== "undefined" && new URLSearchParams(window.location.search).has("mock")) ||
  import.meta.env.BASE_URL.includes("/enbu/");

export interface DeviceStart {
  session_id: string;
  user_code: string;
  verification_uri: string;
  expires_at: string;
  interval: number;
  copied: boolean;
  browser_opened: boolean;
}

export interface DeviceStatus {
  state: "pending" | "success" | "expired" | "denied" | "error";
  message?: string;
  username?: string;
}

type DesktopService = {
  GetAuthStatus: () => Promise<DesktopAuthStatus>;
  StartDeviceLogin: () => Promise<DeviceStart>;
  GetDeviceLoginStatus: (sessionID: string) => Promise<DeviceStatus>;
  CancelDeviceLogin: (sessionID: string) => Promise<void>;
  Logout: () => Promise<void>;
  BrowseRepository: () => Promise<GUIRepoStatus["repo"]>;
  SelectRepository: (path: string) => Promise<GUIRepoStatus["repo"]>;
  GetRepoStatus: () => Promise<GUIRepoStatus["repo"]>;
  Initialize: () => Promise<InitResult>;
  ListEnvironments: () => Promise<Environment[]>;
  CreateEnvironment: (name: string) => Promise<void>;
  SwitchEnvironment: (name: string) => Promise<void>;
  RenameEnvironment: (name: string, newName: string) => Promise<void>;
  DeleteEnvironment: (name: string) => Promise<void>;
  ListSecrets: (env: string) => Promise<SecretsResponse>;
  AddSecret: (env: string, key: string, value: string) => Promise<void>;
  EditSecret: (env: string, key: string, value: string) => Promise<void>;
  DeleteSecret: (env: string, key: string) => Promise<void>;
  PullSecrets: (env: string) => Promise<void>;
  SyncSecrets: (env: string) => Promise<void>;
};

type DesktopAuthStatus = Omit<AuthStatus, "repo"> & {
  repo?: AuthStatus["repo"] & {
    repo?: string;
  };
};

declare global {
  interface Window {
    go?: {
      main?: {
        DesktopService?: DesktopService;
      };
      desktop?: {
        Service?: DesktopService;
      };
    };
  }
}

function service(): DesktopService | undefined {
  return window.go?.main?.DesktopService ?? window.go?.desktop?.Service;
}

const realBackend = {
  async authStatus(): Promise<AuthStatus> {
    const svc = service();
    if (!svc) {
      return api.auth.status();
    }
    return normalizeAuthStatus(await svc.GetAuthStatus());
  },
  async startDeviceLogin(): Promise<DeviceStart> {
    const svc = service();
    if (!svc) {
      throw new Error("Desktop authentication is not available");
    }
    return svc.StartDeviceLogin();
  },
  async deviceStatus(sessionID: string): Promise<DeviceStatus> {
    const svc = service();
    if (!svc) {
      throw new Error("Desktop authentication is not available");
    }
    return svc.GetDeviceLoginStatus(sessionID);
  },
  async cancelDeviceLogin(sessionID: string): Promise<void> {
    await service()?.CancelDeviceLogin(sessionID);
  },
  async logout(): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.auth.logout();
      return;
    }
    await svc.Logout();
  },
  async repoStatus(): Promise<GUIRepoStatus> {
    const svc = service();
    if (!svc) {
      return api.gui.repo();
    }
    const repo = await svc.GetRepoStatus();
    return { selected: Boolean(repo?.path), repo };
  },
  async browseRepository(): Promise<GUIRepoStatus> {
    const svc = service();
    if (!svc) {
      throw new Error("Native repository picker is not available");
    }
    const repo = await svc.BrowseRepository();
    return { selected: Boolean(repo?.path), repo };
  },
  async selectRepository(path: string): Promise<GUIRepoStatus> {
    const svc = service();
    if (!svc) {
      return api.gui.selectRepo(path);
    }
    const repo = await svc.SelectRepository(path);
    return { selected: Boolean(repo?.path), repo };
  },
  async initialize(): Promise<InitResult> {
    const svc = service();
    if (!svc) {
      return api.init();
    }
    return svc.Initialize();
  },
  async listEnvironments(): Promise<Environment[]> {
    const svc = service();
    if (!svc) {
      return (await api.environments.list()).environments;
    }
    return svc.ListEnvironments();
  },
  async createEnvironment(name: string): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.environments.create(name);
      return;
    }
    await svc.CreateEnvironment(name);
  },
  async switchEnvironment(name: string): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.environments.switch(name);
      return;
    }
    await svc.SwitchEnvironment(name);
  },
  async renameEnvironment(name: string, newName: string): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.environments.rename(name, newName);
      return;
    }
    await svc.RenameEnvironment(name, newName);
  },
  async deleteEnvironment(name: string): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.environments.delete(name);
      return;
    }
    await svc.DeleteEnvironment(name);
  },
  async listSecrets(env = ""): Promise<SecretsResponse> {
    return service()?.ListSecrets(env) ?? api.secrets.list(env || undefined);
  },
  async addSecret(key: string, value: string, env = ""): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.secrets.add(key, value, env || undefined);
      return;
    }
    await svc.AddSecret(env, key, value);
  },
  async editSecret(key: string, value: string, env = ""): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.secrets.edit(key, value, env || undefined);
      return;
    }
    await svc.EditSecret(env, key, value);
  },
  async deleteSecret(key: string, env = ""): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.secrets.delete(key, env || undefined);
      return;
    }
    await svc.DeleteSecret(env, key);
  },
  async pullSecrets(env = ""): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.secrets.pull(env || undefined);
      return;
    }
    await svc.PullSecrets(env);
  },
  async syncSecrets(env = ""): Promise<void> {
    const svc = service();
    if (!svc) {
      await api.secrets.sync(env || undefined);
      return;
    }
    await svc.SyncSecrets(env);
  },
};

export const backend = isMock ? mockBackend : realBackend;

export function openURL(url: string): void {
  if (service()) {
    // Wails: use runtime to open in OS browser instead of spawning a new app window
    void import("../wailsjs/runtime/runtime").then(({ BrowserOpenURL }) => BrowserOpenURL(url));
  } else {
    window.open(url, "_blank");
  }
}

function normalizeAuthStatus(status: DesktopAuthStatus): AuthStatus {
  if (!status.repo) {
    return status;
  }
  return {
    ...status,
    repo: {
      owner: status.repo.owner,
      name: status.repo.name ?? status.repo.repo ?? "",
    },
  };
}
