import { createRootRoute, Outlet } from "@tanstack/react-router";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Box, Flex, VStack, styled } from "styled-system/jsx";
import { Button, Popover, Separator, Text } from "../components/ui";
import { NativeSelect } from "../components/ui/native-select";
import { AlertTriangle, Check, Plus, Trash2 } from "lucide-react";
import type { AuthStatus } from "../lib/api";
import { backend, openURL, type GitHubAccount } from "../lib/backend";
import { createAuthRefresher, type AuthRefreshOptions } from "../lib/auth-refresh";
import { I18nProvider, useI18n, type Locale } from "../lib/i18n";
import { ConfirmDeleteDialog } from "../components/confirm-delete-dialog";

export type AuthContextValue = {
  status: AuthStatus | null;
  loading: boolean;
  repoPath: string;
  refresh: (options?: AuthRefreshOptions) => Promise<void> | undefined;
};

export const AuthContext = createContext<AuthContextValue>({
  status: null,
  loading: true,
  repoPath: "",
  refresh: async () => {},
});

export function useAuth() {
  return useContext(AuthContext);
}

export const Route = createRootRoute({
  component: RootWithProviders,
});

function RootWithProviders() {
  return (
    <I18nProvider>
      <RootLayout />
    </I18nProvider>
  );
}

function RootLayout() {
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [repoPath, setRepoPath] = useState("");

  const refresher = useRef(
    createAuthRefresher({
      fetchStatus: () => backend.authStatus(),
      setStatus,
    }),
  );

  const refresh = useCallback((options?: AuthRefreshOptions) => {
    return refresher.current.refresh(options);
  }, []);

  const refreshNow = useCallback(() => {
    return refresh({ force: true });
  }, [refresh]);

  const authContext = useMemo(
    () => ({ status, loading, repoPath, refresh }),
    [status, loading, repoPath, refresh],
  );

  useEffect(() => {
    const init = async () => {
      try {
        const rs = await backend.repoStatus();
        setRepoPath(rs.repo?.path ?? "");
      } catch {
        // ignore
      }
      await refreshNow()?.finally(() => setLoading(false));
    };
    void init();
    const onFocus = () => void refresh();
    const onAuthChanged = () => void refreshNow();
    const onRepoChanged = async () => {
      try {
        const rs = await backend.repoStatus();
        setRepoPath(rs.repo?.path ?? "");
      } catch {
        // ignore
      }
    };
    window.addEventListener("focus", onFocus);
    window.addEventListener("enbu-auth-changed", onAuthChanged);
    window.addEventListener("enbu-repo-changed", onRepoChanged);
    return () => {
      window.removeEventListener("focus", onFocus);
      window.removeEventListener("enbu-auth-changed", onAuthChanged);
      window.removeEventListener("enbu-repo-changed", onRepoChanged);
    };
  }, [refresh, refreshNow]);

  return (
    <AuthContext.Provider value={authContext}>
      <styled.header
        display="flex"
        h="72px"
        alignItems="center"
        justifyContent="space-between"
        px={{ base: "4", md: "5.5" }}
        bg="bg.surface"
        borderBottomWidth="1px"
        borderColor="border.default"
        position="sticky"
        top="0"
        zIndex="10"
      >
        <Flex align="center" gap="2">
          <Text as="span" fontSize="2xl" aria-hidden="true">
            💃
          </Text>
          <Text fontWeight="extrabold" fontSize="2xl" letterSpacing="tight">
            enbu
          </Text>
        </Flex>
        <AccountMenu status={status} loading={loading} />
      </styled.header>
      <Flex minH="calc(100vh - 72px)">
        {status?.authenticated && <Sidebar activePath={repoPath} />}
        <Box flex="1" minW="0">
          <Outlet />
        </Box>
      </Flex>
    </AuthContext.Provider>
  );
}

type RepoItem = NonNullable<
  NonNullable<Awaited<ReturnType<typeof backend.listRepositories>>>[number]
>;

export function Sidebar({ activePath }: { activePath: string }) {
  const { t } = useI18n();
  const [repos, setRepos] = useState<RepoItem[]>([]);
  const [addLoading, setAddLoading] = useState(false);
  const [removingRepository, setRemovingRepository] = useState(false);
  const [repositoryPendingRemoval, setRepositoryPendingRemoval] = useState<RepoItem | null>(null);
  const [contextMenu, setContextMenu] = useState<{
    repo: RepoItem;
    x: number;
    y: number;
  } | null>(null);

  const loadRepos = useCallback(async () => {
    try {
      const list = await backend.listRepositories();
      const seen = new Set<string>();
      setRepos(
        (list ?? []).filter((repo): repo is RepoItem => {
          if (repo == null) return false;
          const key = (repo.path ?? "").replaceAll("\\", "/").toLowerCase();
          if (seen.has(key)) return false;
          seen.add(key);
          return true;
        }),
      );
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    void loadRepos();
    const onChanged = () => void loadRepos();
    window.addEventListener("enbu-repo-changed", onChanged);
    return () => window.removeEventListener("enbu-repo-changed", onChanged);
  }, [loadRepos]);

  useEffect(() => {
    if (!contextMenu) return;
    const close = () => setContextMenu(null);
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    window.addEventListener("pointerdown", close);
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close, true);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("pointerdown", close);
      window.removeEventListener("resize", close);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [contextMenu]);

  return (
    <styled.nav
      w="248px"
      minH="calc(100vh - 72px)"
      display={{ base: "none", lg: "block" }}
      borderRightWidth="1px"
      borderColor="border.default"
      bg="bg.surface"
      p="3.5"
      flexShrink="0"
    >
      <VStack alignItems="stretch" gap="1" mb="3">
        {repos.length === 0 && (
          <Text fontSize="sm" color="fg.muted">
            {t("sidebar.noRepositories")}
          </Text>
        )}
        {repos.map((repo) => {
          const isActive = repo.path === activePath;
          return (
            <Flex
              key={repo.path}
              data-repository-path={repo.path}
              alignItems="center"
              px="2"
              py="9px"
              borderRadius="lg"
              borderWidth="1px"
              borderColor={isActive ? "brand.100" : "transparent"}
              bg={isActive ? "accent.subtle" : undefined}
              _hover={{ bg: isActive ? "accent.subtle" : "bg.muted" }}
              cursor={isActive ? "default" : "pointer"}
              onContextMenu={(event) => {
                event.preventDefault();
                setContextMenu({
                  repo,
                  x: Math.max(8, Math.min(event.clientX, window.innerWidth - 180)),
                  y: Math.max(8, Math.min(event.clientY, window.innerHeight - 56)),
                });
              }}
              onClick={async () => {
                if (isActive) return;
                try {
                  await backend.selectRepository(repo.path ?? "");
                  window.dispatchEvent(new Event("enbu-repo-changed"));
                  window.dispatchEvent(new Event("enbu-auth-changed"));
                } catch {
                  // ignore
                }
              }}
            >
              <Text
                minW="0"
                flex="1"
                fontSize="sm"
                fontWeight={isActive ? "semibold" : "normal"}
                truncate
              >
                {repo.owner}/{repo.repo}
              </Text>
            </Flex>
          );
        })}
      </VStack>
      {contextMenu && (
        <RepositoryContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          removing={removingRepository}
          onRemove={() => {
            setRepositoryPendingRemoval(contextMenu.repo);
            setContextMenu(null);
          }}
        />
      )}
      <ConfirmDeleteDialog
        open={repositoryPendingRemoval != null}
        title={t("sidebar.removeConfirm", {
          repository: repositoryPendingRemoval
            ? `${repositoryPendingRemoval.owner}/${repositoryPendingRemoval.repo}`
            : "",
        })}
        cancelLabel={t("config.cancel")}
        confirmLabel={t("sidebar.remove")}
        loading={removingRepository}
        onClose={() => setRepositoryPendingRemoval(null)}
        onConfirm={async () => {
          if (!repositoryPendingRemoval) return;
          const removedPath = repositoryPendingRemoval.path ?? "";
          const removingActive = removedPath === activePath;
          setRemovingRepository(true);
          try {
            await backend.removeRepository(removedPath);
            const removedKey = removedPath.replaceAll("\\", "/").toLowerCase();
            setRepos((current) =>
              current.filter(
                (repo) => (repo.path ?? "").replaceAll("\\", "/").toLowerCase() !== removedKey,
              ),
            );
            setRepositoryPendingRemoval(null);
            window.dispatchEvent(new Event("enbu-repo-changed"));
            if (removingActive) window.dispatchEvent(new Event("enbu-auth-changed"));
            await loadRepos();
          } finally {
            setRemovingRepository(false);
          }
        }}
      />
      <Button
        size="sm"
        variant="outline"
        w="full"
        borderColor="border.default"
        color="fg.default"
        fontWeight="semibold"
        loading={addLoading}
        onClick={async () => {
          setAddLoading(true);
          try {
            await backend.browseRepository();
            window.dispatchEvent(new Event("enbu-repo-changed"));
            window.dispatchEvent(new Event("enbu-auth-changed"));
            await loadRepos();
          } catch {
            // ignore (user cancelled picker)
          } finally {
            setAddLoading(false);
          }
        }}
      >
        {t("sidebar.addRepository")}
      </Button>
    </styled.nav>
  );
}

export function RepositoryContextMenu({
  x,
  y,
  removing,
  onRemove,
}: {
  x: number;
  y: number;
  removing: boolean;
  onRemove: () => void | Promise<void>;
}) {
  const { t } = useI18n();
  return (
    <Box
      role="menu"
      position="fixed"
      style={{ left: x, top: y }}
      zIndex="40"
      minW="168px"
      p="1"
      bg="bg.surface"
      borderWidth="1px"
      borderColor="border.default"
      borderRadius="lg"
      boxShadow="dropdown"
      onPointerDown={(event) => event.stopPropagation()}
    >
      <Button
        role="menuitem"
        type="button"
        variant="ghost"
        w="full"
        justifyContent="start"
        color="status.danger"
        loading={removing}
        onClick={() => void onRemove()}
      >
        <Trash2 size={15} />
        {t("sidebar.remove")}
      </Button>
    </Box>
  );
}

function AccountContextMenu({
  account,
  x,
  y,
  onRemove,
}: {
  account: GitHubAccount;
  x: number;
  y: number;
  onRemove: () => void;
}) {
  const { t } = useI18n();
  return (
    <Box
      role="menu"
      position="fixed"
      style={{ left: x, top: y }}
      zIndex="40"
      minW="168px"
      p="1"
      bg="bg.surface"
      borderWidth="1px"
      borderColor="border.default"
      borderRadius="lg"
      boxShadow="dropdown"
      onPointerDown={(event) => event.stopPropagation()}
    >
      <Button
        role="menuitem"
        type="button"
        variant="ghost"
        w="full"
        justifyContent="start"
        color="status.danger"
        onClick={onRemove}
      >
        <Trash2 size={15} />
        {t("auth.removeAccount", { username: account.username })}
      </Button>
    </Box>
  );
}

export function AccountMenu({ status, loading }: { status: AuthStatus | null; loading: boolean }) {
  const { locale, setLocale, t } = useI18n();
  const triggerRef = useRef<HTMLButtonElement>(null);

  const authenticated = Boolean(status?.authenticated);
  const username = status?.username ?? "";
  const initial = username.slice(0, 1).toUpperCase() || (loading ? "…" : "-");
  const [accounts, setAccounts] = useState<GitHubAccount[]>([]);
  const [switchingAccount, setSwitchingAccount] = useState("");
  const [accountPendingRemoval, setAccountPendingRemoval] = useState<GitHubAccount | null>(null);
  const [removingAccount, setRemovingAccount] = useState(false);
  const [accountContextMenu, setAccountContextMenu] = useState<{
    account: GitHubAccount;
    x: number;
    y: number;
  } | null>(null);

  const loadAccounts = useCallback(async () => {
    try {
      setAccounts(await backend.listAccounts());
    } catch {
      setAccounts([]);
    }
  }, []);

  useEffect(() => {
    void loadAccounts();
  }, [authenticated, username, loadAccounts]);

  useEffect(() => {
    if (!accountContextMenu) return;
    const close = () => setAccountContextMenu(null);
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    window.addEventListener("pointerdown", close);
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close, true);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("pointerdown", close);
      window.removeEventListener("resize", close);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [accountContextMenu]);

  return (
    <>
      <Popover.Root>
        <Popover.Trigger asChild>
          <Button
            ref={triggerRef}
            variant="ghost"
            w="38px"
            h="38px"
            p="0"
            borderRadius="full"
            aria-label="Account menu"
            _hover={{ bg: "bg.muted", borderWidth: "1px", borderColor: "border.default" }}
          >
            <Box position="relative" w="32px" h="32px" flexShrink="0">
              {authenticated && username ? (
                <img
                  src={`https://avatars.githubusercontent.com/${username}?size=64`}
                  width={32}
                  height={32}
                  style={{ borderRadius: "50%", position: "absolute", top: 0, left: 0 }}
                  onError={(e) => {
                    e.currentTarget.style.display = "none";
                    const fallback = e.currentTarget.nextElementSibling as HTMLElement | null;
                    if (fallback) fallback.style.display = "grid";
                  }}
                  alt={username}
                />
              ) : null}
              <Box
                w="32px"
                h="32px"
                borderRadius="full"
                bg={authenticated ? "accent.default" : "bg.muted"}
                color={authenticated ? "white" : "fg.muted"}
                fontSize="sm"
                fontWeight="extrabold"
                display={authenticated && username ? "none" : "grid"}
                placeItems="center"
                position="absolute"
                top="0"
                left="0"
              >
                {initial}
              </Box>
            </Box>
          </Button>
        </Popover.Trigger>

        <Popover.Positioner>
          <Popover.Content
            w="248px"
            p="3"
            borderWidth="1px"
            borderColor="border.default"
            borderRadius="lg"
            boxShadow="dropdown"
            bg="bg.surface"
          >
            {/* Account name */}
            <Box pb="2" mb="1" borderBottomWidth="1px" borderColor="border.default">
              {authenticated ? (
                <Text
                  fontWeight="bold"
                  fontSize="sm"
                  cursor="pointer"
                  _hover={{ textDecoration: "underline", color: "accent.default" }}
                  onClick={() => openURL(`https://github.com/${username}`)}
                >
                  {username}
                </Text>
              ) : (
                <Text fontWeight="bold" fontSize="sm">
                  Account
                </Text>
              )}
            </Box>

            <>
              <Text mt="2" mb="1" fontSize="2xs" color="fg.muted" fontWeight="bold">
                {t("auth.accounts")}
              </Text>
              <VStack alignItems="stretch" gap="1">
                {accounts.map((account) => (
                  <Flex
                    key={account.id}
                    data-account-id={account.id}
                    align="center"
                    onContextMenu={(event) => {
                      if (account.source !== "stored") return;
                      event.preventDefault();
                      event.stopPropagation();
                      setAccountContextMenu({
                        account,
                        x: Math.max(8, Math.min(event.clientX, window.innerWidth - 180)),
                        y: Math.max(8, Math.min(event.clientY, window.innerHeight - 56)),
                      });
                    }}
                  >
                    <Button
                      type="button"
                      variant="ghost"
                      flex="1"
                      minW="0"
                      h="40px"
                      px="2"
                      justifyContent="space-between"
                      bg={account.active ? "accent.subtle" : undefined}
                      loading={switchingAccount === account.id}
                      disabled={account.active || switchingAccount !== ""}
                      onClick={async () => {
                        setSwitchingAccount(account.id);
                        try {
                          await backend.switchAccount(account.id);
                          await loadAccounts();
                          window.dispatchEvent(new Event("enbu-auth-changed"));
                        } finally {
                          setSwitchingAccount("");
                        }
                      }}
                    >
                      <Flex align="center" gap="2" minW="0">
                        <img
                          src={`https://avatars.githubusercontent.com/${encodeURIComponent(account.username)}?size=48`}
                          width={24}
                          height={24}
                          style={{ borderRadius: "50%" }}
                          alt=""
                        />
                        <Text fontSize="sm" truncate>
                          {account.username}
                        </Text>
                        {account.storage === "file" && (
                          <Box display="flex" title={t("auth.fileStorageWarning")}>
                            <AlertTriangle
                              size={14}
                              color="var(--colors-status-warning)"
                              aria-label={t("auth.fileStorageWarning")}
                            />
                          </Box>
                        )}
                      </Flex>
                      {account.active && <Check size={15} />}
                    </Button>
                  </Flex>
                ))}
                <Button
                  type="button"
                  variant="ghost"
                  w="full"
                  h="38px"
                  px="2"
                  justifyContent="start"
                  color="accent.default"
                  onClick={() => window.dispatchEvent(new Event("enbu-connect-github"))}
                >
                  <Plus size={15} />
                  {t("auth.addAccount")}
                </Button>
              </VStack>
              <Separator my="2" />
            </>

            {/* Language */}
            <Flex justifyContent="space-between" alignItems="center" py="10px">
              <Text fontSize="sm" color="fg.muted">
                {t("app.language")}
              </Text>
              <NativeSelect
                value={locale}
                onChange={(e) => setLocale(e.target.value as Locale)}
                style={{ width: "118px" }}
              >
                <option value="en">English</option>
                <option value="ja">日本語</option>
              </NativeSelect>
            </Flex>
          </Popover.Content>
        </Popover.Positioner>
      </Popover.Root>
      {accountContextMenu && (
        <AccountContextMenu
          account={accountContextMenu.account}
          x={accountContextMenu.x}
          y={accountContextMenu.y}
          onRemove={() => {
            setAccountPendingRemoval(accountContextMenu.account);
            setAccountContextMenu(null);
          }}
        />
      )}
      <ConfirmDeleteDialog
        open={accountPendingRemoval != null}
        title={t("auth.removeAccountConfirm", {
          username: accountPendingRemoval?.username ?? "",
        })}
        cancelLabel={t("config.cancel")}
        confirmLabel={t("sidebar.remove")}
        loading={removingAccount}
        onClose={() => setAccountPendingRemoval(null)}
        onConfirm={async () => {
          if (!accountPendingRemoval) return;
          setRemovingAccount(true);
          try {
            await backend.removeAccount(accountPendingRemoval.id);
            setAccountPendingRemoval(null);
            await loadAccounts();
            window.dispatchEvent(new Event("enbu-auth-changed"));
          } finally {
            setRemovingAccount(false);
          }
        }}
      />
    </>
  );
}
