import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Badge,
  Box,
  Button,
  Code,
  Heading,
  HStack,
  Input,
  Spinner,
  Text,
  VStack,
} from "@chakra-ui/react";
import { backend, type DeviceStart, type DeviceStatus } from "../lib/backend";
import type { AuthStatus, GUIRepoStatus } from "../lib/api";
import { useI18n } from "../lib/i18n";

export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  const { t } = useI18n();
  const [repoStatus, setRepoStatus] = useState<GUIRepoStatus | null>(null);
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [repoPath, setRepoPath] = useState("");
  const [repoError, setRepoError] = useState("");
  const [selectingRepo, setSelectingRepo] = useState(false);
  const [deviceStart, setDeviceStart] = useState<DeviceStart | null>(null);
  const [deviceStatus, setDeviceStatus] = useState<DeviceStatus | null>(null);
  const [authError, setAuthError] = useState("");
  const [startingAuth, setStartingAuth] = useState(false);
  const [now, setNow] = useState(() => Date.now());

  async function refresh() {
    const repo = await backend.repoStatus();
    setRepoStatus(repo);
    if (repo.selected) {
      const auth = await backend.authStatus().catch(() => ({ authenticated: false }));
      setStatus(auth);
    }
  }

  useEffect(() => {
    refresh()
      .catch((err) => setRepoError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!deviceStart || deviceStatus?.state === "success") {
      return;
    }

    const poll = window.setInterval(
      async () => {
        const next = await backend.deviceStatus(deviceStart.session_id);
        setDeviceStatus(next);
        if (next.state === "success") {
          window.clearInterval(poll);
          setStatus(await backend.authStatus());
        }
      },
      Math.max(deviceStart.interval, 2) * 1000,
    );

    return () => window.clearInterval(poll);
  }, [deviceStart, deviceStatus?.state]);

  const expiresIn = useMemo(() => {
    if (!deviceStart) {
      return 0;
    }
    return Math.max(0, Math.ceil((new Date(deviceStart.expires_at).getTime() - now) / 1000));
  }, [deviceStart, now]);

  if (loading) {
    return (
      <VStack py={20}>
        <Spinner size="xl" />
        <Text color="gray.600">{t("common.loading")}</Text>
      </VStack>
    );
  }

  if (!repoStatus?.selected) {
    return (
      <VStack gap={5} align="stretch" maxW="xl" mx="auto" py={16}>
        <Box>
          <Heading size="lg">{t("repo.selectTitle")}</Heading>
          <Text color="gray.600" mt={2}>
            {t("repo.selectDescription")}
          </Text>
        </Box>
        {repoError && (
          <Alert.Root status="error">
            <Alert.Indicator />
            <Alert.Content>{repoError}</Alert.Content>
          </Alert.Root>
        )}
        <HStack align="stretch">
          <Input
            value={repoPath}
            onChange={(event) => setRepoPath(event.target.value)}
            placeholder={t("repo.pathPlaceholder")}
          />
          <Button
            variant="outline"
            onClick={async () => {
              setSelectingRepo(true);
              setRepoError("");
              try {
                const repo = await backend.browseRepository();
                setRepoStatus(repo);
                if (repo.selected) {
                  setStatus(await backend.authStatus().catch(() => ({ authenticated: false })));
                }
              } catch (err) {
                setRepoError(err instanceof Error ? err.message : String(err));
              } finally {
                setSelectingRepo(false);
              }
            }}
          >
            {t("repo.browse")}
          </Button>
        </HStack>
        <Button
          colorScheme="blue"
          loading={selectingRepo}
          onClick={async () => {
            setSelectingRepo(true);
            setRepoError("");
            try {
              const repo = await backend.selectRepository(repoPath);
              setRepoStatus(repo);
              const auth = await backend.authStatus().catch(() => ({ authenticated: false }));
              setStatus(auth);
            } catch (err) {
              setRepoError(err instanceof Error ? err.message : String(err));
            } finally {
              setSelectingRepo(false);
            }
          }}
        >
          {t("repo.continue")}
        </Button>
      </VStack>
    );
  }

  if (!status?.authenticated) {
    return (
      <VStack gap={6} py={16} maxW="xl" mx="auto" align="stretch">
        <Box textAlign="center">
          <Heading size="lg">{t("auth.welcome")}</Heading>
          <Text color="gray.600" mt={2}>
            {t("auth.tagline")}
          </Text>
          {repoStatus.repo && (
            <Text color="gray.600" mt={2}>
              {t("repo.current", { owner: repoStatus.repo.owner, repo: repoStatus.repo.repo })}
            </Text>
          )}
        </Box>

        {authError && (
          <Alert.Root status="error">
            <Alert.Indicator />
            <Alert.Content>{authError}</Alert.Content>
          </Alert.Root>
        )}

        {!deviceStart ? (
          <Button
            colorScheme="blue"
            size="lg"
            loading={startingAuth}
            onClick={async () => {
              setStartingAuth(true);
              setAuthError("");
              try {
                const start = await backend.startDeviceLogin();
                setDeviceStart(start);
                setDeviceStatus({ state: "pending" });
              } catch (err) {
                setAuthError(err instanceof Error ? err.message : String(err));
              } finally {
                setStartingAuth(false);
              }
            }}
          >
            {t("auth.connect")}
          </Button>
        ) : (
          <DeviceLoginPanel
            start={deviceStart}
            status={deviceStatus}
            expiresIn={expiresIn}
            onCancel={async () => {
              await backend.cancelDeviceLogin(deviceStart.session_id);
              setDeviceStart(null);
              setDeviceStatus(null);
            }}
            onRetry={() => {
              setDeviceStart(null);
              setDeviceStatus(null);
              setAuthError("");
            }}
          />
        )}
      </VStack>
    );
  }

  return (
    <VStack gap={6} align="stretch">
      <Box>
        <Heading size="md">{t("auth.hello", { username: status.username ?? "" })}</Heading>
        {status.repo && (
          <Text color="gray.600">
            {t("repo.current", { owner: status.repo.owner, repo: status.repo.name })}
          </Text>
        )}
      </Box>
    </VStack>
  );
}

function DeviceLoginPanel({
  start,
  status,
  expiresIn,
  onCancel,
  onRetry,
}: {
  start: DeviceStart;
  status: DeviceStatus | null;
  expiresIn: number;
  onCancel: () => Promise<void>;
  onRetry: () => void;
}) {
  const { t } = useI18n();
  const terminal = status?.state && status.state !== "pending";

  return (
    <VStack gap={4} align="stretch">
      <Box textAlign="center" bg="white" borderWidth="1px" p={6}>
        <Text color="gray.600">{t("auth.userCode")}</Text>
        <Code fontSize="3xl" p={4} mt={3}>
          {start.user_code}
        </Code>
        <HStack justify="center" mt={3}>
          <Badge colorPalette={start.copied ? "green" : "orange"}>
            {start.copied ? t("auth.copied") : t("auth.copyFailed")}
          </Badge>
          <Badge colorPalette={start.browser_opened ? "green" : "orange"}>
            {start.browser_opened ? t("auth.browserOpened") : t("auth.browserNotOpened")}
          </Badge>
        </HStack>
      </Box>

      {status?.state === "pending" && (
        <HStack justify="center">
          <Spinner />
          <Text>{t("auth.waiting")}</Text>
          <Text color="gray.600">{t("auth.expiresIn", { seconds: expiresIn })}</Text>
        </HStack>
      )}
      {status?.state === "denied" && <AlertText message={t("auth.denied")} />}
      {status?.state === "expired" && <AlertText message={t("auth.expired")} />}
      {status?.state === "error" && <AlertText message={status.message ?? t("auth.error")} />}

      <HStack justify="center" wrap="wrap">
        <Button variant="outline" onClick={() => navigator.clipboard.writeText(start.user_code)}>
          {t("auth.copyCode")}
        </Button>
        <Button variant="outline" onClick={() => window.open(start.verification_uri, "_blank")}>
          {t("auth.openGitHub")}
        </Button>
        {terminal ? (
          <Button colorScheme="blue" onClick={onRetry}>
            {t("auth.tryAgain")}
          </Button>
        ) : (
          <Button variant="ghost" onClick={onCancel}>
            {t("auth.cancel")}
          </Button>
        )}
      </HStack>
    </VStack>
  );
}

function AlertText({ message }: { message: string }) {
  return (
    <Alert.Root status="error">
      <Alert.Indicator />
      <Alert.Content>{message}</Alert.Content>
    </Alert.Root>
  );
}
