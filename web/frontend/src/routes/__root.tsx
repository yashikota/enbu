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
import {
  Badge,
  Box,
  Button,
  Flex,
  NativeSelect,
  Popover,
  Separator,
  Text,
  VStack,
} from "@chakra-ui/react";
import { Trash2 } from "lucide-react";
import type { AuthStatus } from "../lib/api";
import { backend, openURL } from "../lib/backend";
import { createAuthRefresher, type AuthRefreshOptions } from "../lib/auth-refresh";
import { I18nProvider, useI18n, type Locale } from "../lib/i18n";

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
    void refreshNow()?.finally(() => setLoading(false));
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
      <Flex
        as="header"
        h="64px"
        align="center"
        justify="space-between"
        px={6}
        bg="bg.surface"
        borderBottomWidth="1px"
        borderColor="border.default"
        position="sticky"
        top={0}
        zIndex={10}
      >
        <Text fontWeight="extrabold" fontSize="2xl">
          💃 enbu
        </Text>
        <AccountMenu status={status} loading={loading} />
      </Flex>
      <Flex minH="calc(100vh - 64px)">
        {status?.authenticated && <Sidebar activePath={repoPath} />}
        <Box flex={1} minW={0}>
          <Outlet />
        </Box>
      </Flex>
    </AuthContext.Provider>
  );
}

type RepoItem = NonNullable<
  NonNullable<Awaited<ReturnType<typeof backend.listRepositories>>>[number]
>;

function Sidebar({ activePath }: { activePath: string }) {
  const { t } = useI18n();
  const [repos, setRepos] = useState<RepoItem[]>([]);
  const [addLoading, setAddLoading] = useState(false);

  const loadRepos = useCallback(async () => {
    try {
      const list = await backend.listRepositories();
      setRepos((list ?? []).filter((r): r is RepoItem => r != null));
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

  return (
    <Box
      as="nav"
      w="240px"
      minH="calc(100vh - 64px)"
      borderRightWidth="1px"
      borderColor="border.default"
      bg="bg.surface"
      p={3}
      flexShrink={0}
    >
      <Text fontWeight="bold" fontSize="xs" color="fg.muted" textTransform="uppercase" mb={2}>
        {t("sidebar.repositories")}
      </Text>
      <VStack align="stretch" gap={1} mb={3}>
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
              align="center"
              justify="space-between"
              px={2}
              py="6px"
              borderRadius="md"
              bg={isActive ? "accent.subtle" : undefined}
              _hover={{ bg: isActive ? "accent.subtle" : "bg.muted" }}
              cursor={isActive ? "default" : "pointer"}
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
              <Box minW={0} flex={1}>
                <Text fontSize="sm" fontWeight={isActive ? "semibold" : "normal"} truncate>
                  {repo.owner}/{repo.repo}
                </Text>
                {isActive && (
                  <Badge colorPalette="accent" fontSize="xs" mt="1px">
                    {t("sidebar.active")}
                  </Badge>
                )}
              </Box>
              <Button
                aria-label={t("sidebar.remove")}
                variant="ghost"
                size="xs"
                color="fg.muted"
                p={1}
                minW="auto"
                h="auto"
                _hover={{ color: "red.500", bg: "red.50" }}
                onClick={async (e) => {
                  e.stopPropagation();
                  await backend.removeRepository(repo.path ?? "");
                  window.dispatchEvent(new Event("enbu-repo-changed"));
                  if (isActive) window.dispatchEvent(new Event("enbu-auth-changed"));
                  await loadRepos();
                }}
              >
                <Trash2 size={14} />
              </Button>
            </Flex>
          );
        })}
      </VStack>
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
    </Box>
  );
}

function AccountMenu({ status, loading }: { status: AuthStatus | null; loading: boolean }) {
  const { locale, setLocale, t } = useI18n();
  const triggerRef = useRef<HTMLButtonElement>(null);

  const authenticated = Boolean(status?.authenticated);
  const username = status?.username ?? "";
  const initial = username.slice(0, 1).toUpperCase() || (loading ? "…" : "-");

  const statusLabel = loading
    ? "Checking..."
    : authenticated
      ? t("auth.signedIn")
      : t("auth.signedOut");
  const badgeColor = loading ? "gray" : authenticated ? "green" : "gray";

  const repoName = status?.repo?.name;
  const repoOwner = status?.repo?.owner;

  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <Button
          ref={triggerRef}
          variant="ghost"
          w="38px"
          h="38px"
          p={0}
          borderRadius="full"
          aria-label="Account menu"
          _hover={{ bg: "bg.muted", borderWidth: "1px", borderColor: "border.default" }}
        >
          <Box position="relative" w="32px" h="32px" flexShrink={0}>
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
              bg={authenticated ? "green.600" : "gray.300"}
              color={authenticated ? "white" : "gray.600"}
              fontSize="sm"
              fontWeight="extrabold"
              display={authenticated && username ? "none" : "grid"}
              placeItems="center"
              position="absolute"
              top={0}
              left={0}
            >
              {initial}
            </Box>
          </Box>
        </Button>
      </Popover.Trigger>

      <Popover.Positioner>
        <Popover.Content
          w="248px"
          p={3}
          borderWidth="1px"
          borderColor="border.default"
          borderRadius="lg"
          boxShadow="dropdown"
          bg="bg.surface"
        >
          {/* Account name */}
          <Box pb={2} mb={1} borderBottomWidth="1px" borderColor="border.default">
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

          {/* Status */}
          <Flex justify="space-between" align="center" py="10px">
            <Text fontSize="sm" color="fg.muted">
              Status
            </Text>
            <Badge
              colorPalette={badgeColor}
              borderRadius="full"
              px={2}
              py="1px"
              fontSize="xs"
              fontWeight="semibold"
            >
              {statusLabel}
            </Badge>
          </Flex>

          <Separator borderColor="border.default" />

          {/* Language */}
          <Flex justify="space-between" align="center" py="10px">
            <Text fontSize="sm" color="fg.muted">
              {t("app.language")}
            </Text>
            <NativeSelect.Root size="sm" w="118px">
              <NativeSelect.Field
                value={locale}
                onChange={(e) => setLocale(e.target.value as Locale)}
                fontSize="sm"
                h="30px"
                borderColor="border.default"
                borderRadius="md"
              >
                <option value="en">English</option>
                <option value="ja">日本語</option>
              </NativeSelect.Field>
              <NativeSelect.Indicator />
            </NativeSelect.Root>
          </Flex>

          {/* Repository info */}
          {authenticated && repoOwner && repoName && (
            <>
              <Separator borderColor="border.default" />
              <Flex justify="space-between" align="center" py="10px">
                <Text fontSize="sm" color="fg.muted">
                  Repository
                </Text>
                <Text
                  fontSize="sm"
                  truncate
                  maxW="120px"
                  cursor="pointer"
                  _hover={{ textDecoration: "underline", color: "accent.default" }}
                  onClick={() => openURL(`https://github.com/${repoOwner}/${repoName}`)}
                >
                  {repoName}
                </Text>
              </Flex>
            </>
          )}

          {/* Action button */}
          <Separator borderColor="border.default" mb={2} />
          {authenticated ? (
            <Button
              size="sm"
              variant="outline"
              w="full"
              borderColor="border.default"
              color="fg.default"
              fontWeight="semibold"
              onClick={async () => {
                await backend.logout();
                window.dispatchEvent(new Event("enbu-auth-changed"));
              }}
            >
              {t("auth.logout")}
            </Button>
          ) : (
            <Button
              size="sm"
              variant="outline"
              w="full"
              borderColor="border.default"
              color="fg.default"
              fontWeight="semibold"
              onClick={() => window.dispatchEvent(new Event("enbu-connect-github"))}
            >
              Connect GitHub
            </Button>
          )}
        </Popover.Content>
      </Popover.Positioner>
    </Popover.Root>
  );
}
