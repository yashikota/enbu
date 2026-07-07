import {
  api,
  type AuthStatus,
  type Environment,
  type GUIRepoStatus,
  type SecretsResponse,
} from "./api";

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
  Initialize: () => Promise<unknown>;
  ListEnvironments: () => Promise<Environment[]>;
  ListSecrets: (env: string) => Promise<SecretsResponse>;
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

export const backend = {
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
};

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
