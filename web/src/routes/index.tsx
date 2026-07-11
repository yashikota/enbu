import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Box, Flex, HStack, VStack, styled } from "styled-system/jsx";
import {
  Alert,
  Button,
  Heading,
  Input,
  Separator,
  Spinner,
  Text,
  Textarea,
} from "../components/ui";
import { Copy, Eye, EyeOff } from "lucide-react";
import { SiGithub } from "@icons-pack/react-simple-icons";
import { backend, type DeviceStart, type DeviceStatus, openURL } from "../lib/backend";
import type { Environment, Recipient, SecretsResponse } from "../lib/api";
import { useI18n } from "../lib/i18n";
import { useAuth } from "./__root";

export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  const { t } = useI18n();
  const { status, loading: authLoading } = useAuth();

  const [repoStatus, setRepoStatus] = useState<{
    selected: boolean;
    repo?: { path?: string; owner: string; repo: string; initialized?: boolean };
  } | null>(null);
  const [repoPath, setRepoPath] = useState("");
  const [repoError, setRepoError] = useState("");
  const [selectingRepo, setSelectingRepo] = useState(false);
  const [deviceStart, setDeviceStart] = useState<DeviceStart | null>(null);
  const [deviceStatus, setDeviceStatus] = useState<DeviceStatus | null>(null);
  const [authError, setAuthError] = useState("");
  const [startingAuth, setStartingAuth] = useState(false);
  const [initializing, setInitializing] = useState(false);
  const [workspaceLoading, setWorkspaceLoading] = useState(false);
  const [pullLoading, setPullLoading] = useState(false);
  const [syncLoading, setSyncLoading] = useState(false);
  const [addLoading, setAddLoading] = useState(false);
  const [actionError, setActionError] = useState("");
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [secrets, setSecrets] = useState<SecretsResponse | null>(null);
  const [secretKey, setSecretKey] = useState("");
  const [secretValue, setSecretValue] = useState("");
  const [newEnv, setNewEnv] = useState("");
  const [now, setNow] = useState(() => Date.now());

  const fetchRepoStatus = useCallback(async () => {
    if (status?.authenticated) {
      setRepoStatus(await backend.repoStatus());
    }
  }, [status?.authenticated]);

  useEffect(() => {
    fetchRepoStatus().catch((err) =>
      setRepoError(err instanceof Error ? err.message : String(err)),
    );
  }, [fetchRepoStatus]);

  useEffect(() => {
    const onAuthChanged = () => void fetchRepoStatus();
    window.addEventListener("enbu-auth-changed", onAuthChanged);
    return () => window.removeEventListener("enbu-auth-changed", onAuthChanged);
  }, [fetchRepoStatus]);

  useEffect(() => {
    const onConnect = () => {
      if (!status?.authenticated && !deviceStart) void handleStartAuth();
    };
    window.addEventListener("enbu-connect-github", onConnect);
    return () => window.removeEventListener("enbu-connect-github", onConnect);
  }, [status?.authenticated, deviceStart]);

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (!deviceStart || deviceStatus?.state === "success") return;
    const poll = window.setInterval(
      async () => {
        const next = await backend.deviceStatus(deviceStart.session_id);
        setDeviceStatus(next);
        if (next.state === "success") {
          window.clearInterval(poll);
          window.dispatchEvent(new Event("enbu-auth-changed"));
          setRepoStatus(await backend.repoStatus());
        }
      },
      Math.max(deviceStart.interval, 2) * 1000,
    );
    return () => window.clearInterval(poll);
  }, [deviceStart, deviceStatus?.state]);

  const expiresIn = useMemo(() => {
    if (!deviceStart) return 0;
    return Math.max(0, Math.ceil((new Date(deviceStart.expires_at).getTime() - now) / 1000));
  }, [deviceStart, now]);

  const currentEnvironment = useMemo(
    () => environments.find((env) => env.current)?.name ?? secrets?.environment ?? "",
    [environments, secrets?.environment],
  );

  async function refreshWorkspace(env = currentEnvironment) {
    setWorkspaceLoading(true);
    setActionError("");
    try {
      const envs = await backend.listEnvironments();
      const nextEnv = env || envs.find((item) => item.current)?.name || "";
      setEnvironments([...envs].sort((a, b) => a.name.localeCompare(b.name)));
      setSecrets(await backend.listSecrets(nextEnv));
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    } finally {
      setWorkspaceLoading(false);
    }
  }

  useEffect(() => {
    if (repoStatus?.selected && repoStatus.repo?.initialized) void refreshWorkspace();
  }, [repoStatus?.selected, repoStatus?.repo?.initialized]);

  async function handleStartAuth() {
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
  }

  // Screen 01: Loading
  if (authLoading) {
    return (
      <PageCenter>
        <VStack gap={3}>
          <Spinner size="xl" color="accent.default" />
          <Text color="fg.muted">{t("common.loading")}</Text>
        </VStack>
      </PageCenter>
    );
  }

  // Screen 02: Auth start
  if (!status?.authenticated && !deviceStart) {
    return (
      <PageCenter>
        <VStack gap={5} w="full" maxW="480px" textAlign="center">
          <Box>
            <Heading size="2xl" fontWeight="extrabold" mb={2}>
              {t("auth.welcome")}
            </Heading>
            <Text color="fg.muted">{t("auth.tagline")}</Text>
          </Box>
          {authError && <ErrorAlert message={authError} />}
          <Button
            w="full"
            bg="accent.default"
            color="accent.fg"
            fontWeight="semibold"
            loading={startingAuth}
            onClick={handleStartAuth}
          >
            <SiGithub size={16} />
            {t("auth.connect")}
          </Button>
        </VStack>
      </PageCenter>
    );
  }

  // Screen 03: Device flow
  if (!status?.authenticated && deviceStart) {
    return (
      <PageCenter>
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
      </PageCenter>
    );
  }

  // Screen 04: Repository selection
  if (!repoStatus?.selected) {
    return (
      <PageCenter>
        <VStack gap={5} w="full" maxW="540px" alignItems="stretch">
          <Box>
            <Heading size="2xl" fontWeight="extrabold" mb={2}>
              {t("repo.selectTitle")}
            </Heading>
            <Text color="fg.muted">{t("repo.selectDescription")}</Text>
          </Box>
          {repoError && <ErrorAlert message={repoError} />}
          <HStack>
            <Input
              value={repoPath}
              onChange={(e) => setRepoPath(e.target.value)}
              placeholder={t("repo.pathPlaceholder")}
              h="38px"
              borderColor="border.default"
              borderRadius="md"
            />
            <Button
              variant="outline"
              h="38px"
              borderColor="border.default"
              color="fg.default"
              fontWeight="semibold"
              flexShrink={0}
              onClick={async () => {
                setSelectingRepo(true);
                setRepoError("");
                try {
                  setRepoStatus(await backend.browseRepository());
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
            bg="accent.default"
            color="accent.fg"
            fontWeight="semibold"
            loading={selectingRepo}
            onClick={async () => {
              setSelectingRepo(true);
              setRepoError("");
              try {
                setRepoStatus(await backend.selectRepository(repoPath));
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
      </PageCenter>
    );
  }

  // Screen 05: Initialize
  if (!repoStatus.repo?.initialized) {
    return (
      <PageCenter>
        <VStack gap={5} w="full" maxW="540px" alignItems="stretch">
          <Box>
            <Heading size="2xl" fontWeight="extrabold" mb={2}>
              {t("init.title")}
            </Heading>
            <Text color="fg.muted">{t("init.description")}</Text>
            <Text color="fg.muted" mt={1}>
              {t("repo.current", {
                owner: repoStatus.repo?.owner ?? "",
                repo: repoStatus.repo?.repo ?? "",
              })}
            </Text>
          </Box>
          {actionError && <ErrorAlert message={actionError} />}
          <Button
            bg="accent.default"
            color="accent.fg"
            fontWeight="semibold"
            loading={initializing}
            onClick={async () => {
              setInitializing(true);
              setActionError("");
              try {
                await backend.initialize();
                setRepoStatus(await backend.repoStatus());
              } catch (err) {
                setActionError(err instanceof Error ? err.message : String(err));
              } finally {
                setInitializing(false);
              }
            }}
          >
            {t("init.action")}
          </Button>
        </VStack>
      </PageCenter>
    );
  }

  // Screen 06/07: Dashboard
  return (
    <Box minH="calc(100vh - 64px)" py={8} px={6} display="grid" placeItems="start center">
      <VStack gap={6} w="full" maxW="880px" alignItems="stretch">
        <Text color="fg.muted">
          {t("repo.current", {
            owner: repoStatus.repo.owner,
            repo: repoStatus.repo.repo,
          })}
        </Text>

        {actionError && <ErrorAlert message={actionError} />}

        {/* Environments panel */}
        <Panel>
          <Heading size="md" fontWeight="bold" mb={3}>
            {t("dashboard.environments")}
          </Heading>
          <HStack flexWrap="wrap" gap={2}>
            {environments.map((env) => (
              <Button
                key={env.name}
                size="sm"
                variant={env.current ? "solid" : "outline"}
                bg={env.current ? "accent.default" : undefined}
                color={env.current ? "accent.fg" : "fg.default"}
                borderColor={env.current ? undefined : "border.default"}
                fontWeight="semibold"
                onClick={async () => {
                  setActionError("");
                  try {
                    await backend.switchEnvironment(env.name);
                    await refreshWorkspace(env.name);
                  } catch (err) {
                    setActionError(err instanceof Error ? err.message : String(err));
                  }
                }}
              >
                {env.name}
              </Button>
            ))}
          </HStack>
          <HStack mt={3}>
            <Input
              value={newEnv}
              onChange={(e) => setNewEnv(e.target.value)}
              placeholder={t("dashboard.newEnvironment")}
              h="38px"
              borderColor="border.default"
              borderRadius="md"
            />
            <Button
              variant="outline"
              h="38px"
              borderColor="border.default"
              color="fg.default"
              fontWeight="semibold"
              flexShrink={0}
              onClick={async () => {
                if (!newEnv.trim()) return;
                setActionError("");
                try {
                  await backend.createEnvironment(newEnv.trim());
                  setNewEnv("");
                  await refreshWorkspace(newEnv.trim());
                } catch (err) {
                  setActionError(err instanceof Error ? err.message : String(err));
                }
              }}
            >
              {t("dashboard.createEnvironment")}
            </Button>
          </HStack>
        </Panel>

        {/* Recipients panel */}
        <RecipientsPanel />

        {/* Config panel */}
        <ConfigPanel />

        {/* Secrets panel */}
        <Panel>
          <Flex justify="space-between" align="center" mb={3}>
            <Heading size="md" fontWeight="bold">
              {t("dashboard.secrets")}
            </Heading>
            <HStack>
              <Button
                size="sm"
                variant="outline"
                borderColor="border.default"
                color="fg.default"
                fontWeight="semibold"
                loading={pullLoading}
                onClick={async () => {
                  setPullLoading(true);
                  try {
                    await backend.pullSecrets(currentEnvironment);
                    await refreshWorkspace(currentEnvironment);
                  } catch (err) {
                    setActionError(err instanceof Error ? err.message : String(err));
                  } finally {
                    setPullLoading(false);
                  }
                }}
              >
                {t("dashboard.pull")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                borderColor="border.default"
                color="fg.default"
                fontWeight="semibold"
                loading={syncLoading}
                onClick={async () => {
                  setSyncLoading(true);
                  try {
                    await backend.syncSecrets(currentEnvironment);
                    await refreshWorkspace(currentEnvironment);
                  } catch (err) {
                    setActionError(err instanceof Error ? err.message : String(err));
                  } finally {
                    setSyncLoading(false);
                  }
                }}
              >
                {t("dashboard.sync")}
              </Button>
            </HStack>
          </Flex>

          {workspaceLoading && (
            <HStack mb={3}>
              <Spinner size="sm" color="accent.default" />
              <Text fontSize="sm" color="fg.muted">
                {t("common.loading")}
              </Text>
            </HStack>
          )}

          {!workspaceLoading && (secrets?.secrets.length ?? 0) === 0 && (
            <Text color="fg.muted" fontSize="sm" py={2}>
              {t("dashboard.empty")}
            </Text>
          )}

          <VStack alignItems="stretch" gap={2}>
            {secrets?.secrets.map((secret) => (
              <SecretRow
                key={secret.key}
                secretKey={secret.key}
                secretValue={secret.value}
                onEdit={async (val) => {
                  try {
                    await backend.editSecret(secret.key, val, currentEnvironment);
                    await refreshWorkspace(currentEnvironment);
                  } catch (err) {
                    setActionError(err instanceof Error ? err.message : String(err));
                  }
                }}
                onDelete={async () => {
                  try {
                    await backend.deleteSecret(secret.key, currentEnvironment);
                    await refreshWorkspace(currentEnvironment);
                  } catch (err) {
                    setActionError(err instanceof Error ? err.message : String(err));
                  }
                }}
                deleteLabel={t("dashboard.delete")}
              />
            ))}
          </VStack>

          <Separator mt={4} mb={3} borderColor="border.default" />

          {/* Add secret row */}
          <Box display="grid" gridTemplateColumns="220px minmax(0,1fr) auto" gap={2}>
            <Input
              placeholder={t("dashboard.key")}
              value={secretKey}
              onChange={(e) => setSecretKey(e.target.value)}
              h="38px"
              borderColor="border.default"
              borderRadius="md"
            />
            <Input
              placeholder={t("dashboard.value")}
              value={secretValue}
              onChange={(e) => setSecretValue(e.target.value)}
              h="38px"
              borderColor="border.default"
              borderRadius="md"
            />
            <Button
              bg="accent.default"
              color="accent.fg"
              fontWeight="semibold"
              h="38px"
              loading={addLoading}
              onClick={async () => {
                if (!secretKey.trim()) return;
                setAddLoading(true);
                try {
                  await backend.addSecret(secretKey.trim(), secretValue, currentEnvironment);
                  setSecretKey("");
                  setSecretValue("");
                  await refreshWorkspace(currentEnvironment);
                } catch (err) {
                  setActionError(err instanceof Error ? err.message : String(err));
                } finally {
                  setAddLoading(false);
                }
              }}
            >
              {t("dashboard.add")}
            </Button>
          </Box>
        </Panel>
      </VStack>
    </Box>
  );
}

// --- Sub-components ---

function PageCenter({ children }: { children: React.ReactNode }) {
  return (
    <Box minH="calc(100vh - 64px)" display="grid" placeItems="center" px={6} py={12}>
      {children}
    </Box>
  );
}

function Panel({ children }: { children: React.ReactNode }) {
  return (
    <Box p="18px" borderWidth="1px" borderColor="border.default" borderRadius="md" bg="bg.surface">
      {children}
    </Box>
  );
}

function ErrorAlert({ message }: { message: string }) {
  return (
    <Alert.Root borderRadius="md" borderWidth="1px" borderColor="red.200" bg="red.50" py={3} px={4}>
      <Alert.Indicator />
      <Alert.Content>
        <Text fontSize="sm">{message}</Text>
      </Alert.Content>
    </Alert.Root>
  );
}

export function SecretRow({
  secretKey,
  secretValue,
  onEdit,
  onDelete,
  deleteLabel,
}: {
  secretKey: string;
  secretValue: string;
  onEdit: (val: string) => Promise<void>;
  onDelete: () => Promise<void>;
  deleteLabel: string;
}) {
  const [visible, setVisible] = useState(false);
  const [deleting, setDeleting] = useState(false);

  return (
    <Box display="grid" gridTemplateColumns="220px minmax(0,1fr) 38px auto" gap={2}>
      <Input
        readOnly
        value={secretKey}
        h="38px"
        borderColor="border.default"
        borderRadius="md"
        bg="bg.muted"
        color="fg.muted"
      />
      <Input
        defaultValue={secretValue}
        type={visible ? "text" : "password"}
        h="38px"
        borderColor="border.default"
        borderRadius="md"
        onBlur={async (e) => {
          if (e.target.value !== secretValue) await onEdit(e.target.value);
        }}
      />
      <Button
        variant="ghost"
        w="38px"
        h="38px"
        p={0}
        color="fg.muted"
        _hover={{ bg: "bg.muted", color: "fg.default" }}
        aria-label={visible ? "Hide value" : "Show value"}
        onClick={() => setVisible((v) => !v)}
      >
        {visible ? <EyeOff size={16} /> : <Eye size={16} />}
      </Button>
      <Button
        variant="outline"
        h="38px"
        borderColor="border.default"
        color="fg.default"
        fontWeight="semibold"
        loading={deleting}
        onClick={async () => {
          setDeleting(true);
          try {
            await onDelete();
          } finally {
            setDeleting(false);
          }
        }}
      >
        {deleteLabel}
      </Button>
    </Box>
  );
}

function RecipientsPanel() {
  const { t } = useI18n();
  const [recipients, setRecipients] = useState<Recipient[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const list = await backend.listRecipients();
      setRecipients((list ?? []).filter((r): r is Recipient => r != null));
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  const toggle = useCallback(async () => {
    if (!open) await load();
    setOpen((v) => !v);
  }, [open, load]);

  return (
    <Panel>
      <Flex justify="space-between" align="center">
        <Heading size="md" fontWeight="bold">
          {t("recipients.title")}
        </Heading>
        <Button
          size="sm"
          variant="outline"
          borderColor="border.default"
          color="fg.default"
          fontWeight="semibold"
          loading={loading && !open}
          onClick={toggle}
        >
          {open
            ? t("config.cancel")
            : t("config.edit").replace("編集", "表示").replace("Edit", "Show")}
        </Button>
      </Flex>
      {open && (
        <Box mt={3}>
          {loading ? (
            <HStack>
              <Spinner size="sm" color="accent.default" />
              <Text fontSize="sm" color="fg.muted">
                {t("common.loading")}
              </Text>
            </HStack>
          ) : recipients.length === 0 ? (
            <Text fontSize="sm" color="fg.muted">
              {t("recipients.empty")}
            </Text>
          ) : (
            <VStack alignItems="stretch" gap={2}>
              {recipients.map((r) => (
                <Box
                  key={r.fingerprint}
                  p={3}
                  borderWidth="1px"
                  borderColor="border.default"
                  borderRadius="md"
                  bg="bg.muted"
                >
                  <Text fontWeight="semibold" fontSize="sm">
                    {r.username}
                  </Text>
                  <Text fontSize="xs" color="fg.muted" fontFamily="mono" mt="2px">
                    {r.fingerprint}
                  </Text>
                </Box>
              ))}
            </VStack>
          )}
        </Box>
      )}
    </Panel>
  );
}

function ConfigPanel() {
  const { t } = useI18n();
  const [content, setContent] = useState("");
  const [draft, setDraft] = useState("");
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [saveError, setSaveError] = useState("");

  const load = useCallback(async () => {
    try {
      const text = await backend.readConfig();
      setContent(text ?? "");
      setDraft(text ?? "");
      setLoadError("");
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : String(err));
    }
  }, []);

  const toggle = useCallback(async () => {
    if (!open) await load();
    setOpen((v) => !v);
    setEditing(false);
    setSaveError("");
  }, [open, load]);

  return (
    <Panel>
      <Flex justify="space-between" align="center">
        <Heading size="md" fontWeight="bold">
          {t("config.title")}
        </Heading>
        <HStack gap={2}>
          {open && !editing && (
            <Button
              size="sm"
              variant="outline"
              borderColor="border.default"
              color="fg.default"
              fontWeight="semibold"
              onClick={() => {
                setDraft(content);
                setEditing(true);
                setSaveError("");
              }}
            >
              {t("config.edit")}
            </Button>
          )}
          {open && editing && (
            <>
              <Button
                size="sm"
                variant="ghost"
                color="fg.muted"
                onClick={() => {
                  setEditing(false);
                  setDraft(content);
                  setSaveError("");
                }}
              >
                {t("config.cancel")}
              </Button>
              <Button
                size="sm"
                bg="accent.default"
                color="accent.fg"
                fontWeight="semibold"
                loading={saving}
                onClick={async () => {
                  setSaving(true);
                  setSaveError("");
                  try {
                    await backend.writeConfig(draft);
                    setContent(draft);
                    setEditing(false);
                  } catch {
                    setSaveError(t("config.saveError"));
                  } finally {
                    setSaving(false);
                  }
                }}
              >
                {t("config.save")}
              </Button>
            </>
          )}
          <Button
            size="sm"
            variant="outline"
            borderColor="border.default"
            color="fg.default"
            fontWeight="semibold"
            onClick={toggle}
          >
            {open ? t("config.cancel") : t("config.title")}
          </Button>
        </HStack>
      </Flex>
      {open && (
        <Box mt={3}>
          {loadError && <ErrorAlert message={loadError} />}
          {saveError && <ErrorAlert message={saveError} />}
          {editing ? (
            <Textarea
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              fontFamily="mono"
              fontSize="sm"
              minH="240px"
              borderColor="border.default"
              borderRadius="md"
              bg="bg.canvas"
            />
          ) : (
            <styled.pre
              fontSize="sm"
              fontFamily="mono"
              p="3"
              borderWidth="1px"
              borderColor="border.default"
              borderRadius="md"
              bg="bg.muted"
              overflowX="auto"
              whiteSpace="pre-wrap"
            >
              {content || <styled.span color="fg.muted">(empty)</styled.span>}
            </styled.pre>
          )}
        </Box>
      )}
    </Panel>
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
  const [countdown, setCountdown] = useState(5);

  useEffect(() => {
    if (terminal) return;
    if (countdown <= 0) {
      openURL(start.verification_uri);
      return;
    }
    const timer = window.setTimeout(() => setCountdown((c) => c - 1), 1000);
    return () => window.clearTimeout(timer);
  }, [countdown, terminal, start.verification_uri]);

  return (
    <VStack gap={5} w="full" maxW="480px" alignItems="stretch">
      <Heading size="2xl" fontWeight="extrabold" textAlign="center">
        {t("auth.authorizeTitle")}
      </Heading>

      {/* Code copy button */}
      <VStack gap={2} alignItems="center">
        <Text fontSize="sm" color="fg.muted">
          {t("auth.codeInstruction")}
        </Text>
        <Button
          display="inline-flex"
          alignItems="center"
          gap={3}
          h="auto"
          py={4}
          px={5}
          borderRadius="md"
          borderWidth="1px"
          borderColor="border.default"
          color="fg.default"
          bg="gray.50"
          _hover={{ bg: "gray.100" }}
          onClick={() => void navigator.clipboard.writeText(start.user_code)}
          aria-label="Copy device code"
        >
          <Copy size={18} color="#57606a" />
          <Text fontFamily="mono" fontSize="3xl" fontWeight="800" letterSpacing="0.08em">
            {start.user_code}
          </Text>
        </Button>
      </VStack>

      {/* Auto-redirect countdown / status */}
      {!terminal && countdown > 0 && (
        <HStack justifyContent="center">
          <Spinner size="sm" color="accent.default" />
          <Text fontSize="sm" color="fg.muted">
            {t("auth.autoRedirect", { seconds: countdown })}
          </Text>
        </HStack>
      )}
      {!terminal && countdown <= 0 && status?.state === "pending" && (
        <HStack justifyContent="center">
          <Spinner size="sm" color="accent.default" />
          <Text fontSize="sm">{t("auth.waiting")}</Text>
          <Text fontSize="xs" color="fg.muted">
            {t("auth.expiresIn", { seconds: expiresIn })}
          </Text>
        </HStack>
      )}
      {status?.state === "denied" && <ErrorAlert message={t("auth.denied")} />}
      {status?.state === "expired" && <ErrorAlert message={t("auth.expired")} />}
      {status?.state === "error" && <ErrorAlert message={status.message ?? t("auth.error")} />}

      {/* Actions */}
      <HStack justifyContent="center" flexWrap="wrap">
        <Button
          variant="outline"
          h="38px"
          borderColor="border.default"
          color="fg.default"
          fontWeight="semibold"
          onClick={() => openURL(start.verification_uri)}
        >
          {t("auth.openGitHub")}
        </Button>
        {terminal ? (
          <Button
            bg="accent.default"
            color="accent.fg"
            fontWeight="semibold"
            h="38px"
            onClick={onRetry}
          >
            {t("auth.tryAgain")}
          </Button>
        ) : (
          <Button variant="ghost" h="38px" color="fg.muted" onClick={onCancel}>
            {t("auth.cancel")}
          </Button>
        )}
      </HStack>
    </VStack>
  );
}
