import { createRootRoute, Outlet } from "@tanstack/react-router";
import { createContext, useContext, useEffect, useRef, useState } from "react";
import { Badge, Box, Button, Flex, NativeSelect, Popover, Separator, Text } from "@chakra-ui/react";
import type { AuthStatus } from "../lib/api";
import { backend } from "../lib/backend";
import { I18nProvider, useI18n, type Locale } from "../lib/i18n";

export type AuthContextValue = {
  status: AuthStatus | null;
  loading: boolean;
  refresh: () => Promise<void>;
};

export const AuthContext = createContext<AuthContextValue>({
  status: null,
  loading: true,
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

  async function refresh() {
    try {
      setStatus(await backend.authStatus());
    } catch {
      setStatus(null);
    }
  }

  useEffect(() => {
    void refresh().finally(() => setLoading(false));
    const timer = window.setInterval(() => void refresh(), 5000);
    const onFocus = () => void refresh();
    const onAuthChanged = () => void refresh();
    window.addEventListener("focus", onFocus);
    window.addEventListener("enbu-auth-changed", onAuthChanged);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("focus", onFocus);
      window.removeEventListener("enbu-auth-changed", onAuthChanged);
    };
  }, []);

  return (
    <AuthContext.Provider value={{ status, loading, refresh }}>
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
      <Outlet />
    </AuthContext.Provider>
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
          <Box
            w="32px"
            h="32px"
            borderRadius="full"
            bg={authenticated ? "green.600" : "gray.300"}
            color={authenticated ? "white" : "gray.600"}
            fontSize="sm"
            fontWeight="extrabold"
            display="grid"
            placeItems="center"
            flexShrink={0}
          >
            {initial}
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
            <Text fontWeight="bold" fontSize="sm">
              {authenticated ? username : "Account"}
            </Text>
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
                <Text fontSize="sm" truncate maxW="120px">
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
