import { createRootRoute, Outlet } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import {
  Badge,
  Box,
  Button,
  Container,
  Heading,
  Flex,
  HStack,
  NativeSelect,
  Text,
} from "@chakra-ui/react";
import type { AuthStatus } from "../lib/api";
import { backend } from "../lib/backend";
import { I18nProvider, useI18n, type Locale } from "../lib/i18n";

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
  const { locale, setLocale, t } = useI18n();

  return (
    <Box minH="100vh" bg="gray.50">
      <Flex
        as="header"
        bg="white"
        borderBottomWidth="1px"
        py={3}
        px={6}
        align="center"
        justify="space-between"
      >
        <Heading size="md">{t("app.name")}</Heading>
        <HStack gap={3}>
          <AuthStatusChip />
          <NativeSelect.Root width="140px" size="sm">
            <NativeSelect.Field
              aria-label={t("app.language")}
              value={locale}
              onChange={(event) => setLocale(event.target.value as Locale)}
            >
              <option value="en">English</option>
              <option value="ja">日本語</option>
            </NativeSelect.Field>
            <NativeSelect.Indicator />
          </NativeSelect.Root>
        </HStack>
      </Flex>
      <Container maxW="container.lg" py={8}>
        <Outlet />
      </Container>
    </Box>
  );
}

function AuthStatusChip() {
  const { t } = useI18n();
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [error, setError] = useState(false);

  async function refresh() {
    try {
      setStatus(await backend.authStatus());
      setError(false);
    } catch {
      setStatus(null);
      setError(true);
    }
  }

  useEffect(() => {
    void refresh();
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

  const authenticated = Boolean(status?.authenticated);
  const username = status?.username ?? "";
  const initial = username.slice(0, 1).toUpperCase() || "?";

  return (
    <HStack gap={2}>
      <Box
        aria-label={authenticated ? t("auth.signedIn") : t("auth.signedOut")}
        bg={authenticated ? "green.600" : "gray.300"}
        color={authenticated ? "white" : "gray.700"}
        w="32px"
        h="32px"
        borderRadius="full"
        display="grid"
        placeItems="center"
        fontSize="sm"
        fontWeight="bold"
      >
        {authenticated ? initial : "-"}
      </Box>
      <Box minW="92px">
        <Badge colorPalette={error ? "red" : authenticated ? "green" : "gray"}>
          {error ? t("auth.checkFailed") : authenticated ? t("auth.signedIn") : t("auth.signedOut")}
        </Badge>
        {authenticated && (
          <Text fontSize="xs" color="gray.600" mt={1} maxW="120px" truncate>
            {username}
          </Text>
        )}
      </Box>
      {authenticated && (
        <Button
          size="xs"
          variant="ghost"
          onClick={async () => {
            await backend.logout();
            window.dispatchEvent(new Event("enbu-auth-changed"));
          }}
        >
          {t("auth.logout")}
        </Button>
      )}
    </HStack>
  );
}
