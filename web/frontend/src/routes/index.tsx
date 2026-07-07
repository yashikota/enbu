import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { Alert, Box, Button, Heading, Input, Spinner, Text, VStack } from "@chakra-ui/react";
import { api, type AuthStatus, type GUIRepoStatus } from "../lib/api";

export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  const [repoStatus, setRepoStatus] = useState<GUIRepoStatus | null>(null);
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [repoPath, setRepoPath] = useState("");
  const [repoError, setRepoError] = useState("");
  const [selectingRepo, setSelectingRepo] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        const repo = await api.gui.repo();
        setRepoStatus(repo);
        if (repo.selected) {
          const auth = await api.auth.status().catch(() => ({ authenticated: false }));
          setStatus(auth);
        }
      } catch (err) {
        setRepoError(err instanceof Error ? err.message : "Failed to load repository status");
      } finally {
        setLoading(false);
      }
    }
    void load();
  }, []);

  if (loading) {
    return (
      <VStack py={20}>
        <Spinner size="xl" />
      </VStack>
    );
  }

  if (!repoStatus?.selected) {
    return (
      <VStack gap={5} align="stretch" maxW="xl" mx="auto" py={16}>
        <Box>
          <Heading size="lg">Select repository</Heading>
          <Text color="gray.600" mt={2}>
            Choose the local GitHub repository that enbu should manage.
          </Text>
        </Box>
        {repoError && (
          <Alert.Root status="error">
            <Alert.Indicator />
            <Alert.Content>{repoError}</Alert.Content>
          </Alert.Root>
        )}
        <Input
          value={repoPath}
          onChange={(event) => setRepoPath(event.target.value)}
          placeholder="C:\\Users\\you\\src\\your-repo"
        />
        <Button
          colorScheme="blue"
          loading={selectingRepo}
          onClick={async () => {
            setSelectingRepo(true);
            setRepoError("");
            try {
              const repo = await api.gui.selectRepo(repoPath);
              setRepoStatus(repo);
              const auth = await api.auth.status().catch(() => ({ authenticated: false }));
              setStatus(auth);
            } catch (err) {
              setRepoError(err instanceof Error ? err.message : "Failed to select repository");
            } finally {
              setSelectingRepo(false);
            }
          }}
        >
          Continue
        </Button>
      </VStack>
    );
  }

  if (!status?.authenticated) {
    return (
      <VStack gap={6} py={20}>
        <Heading size="lg">Welcome to enbu</Heading>
        <Text color="gray.600">Keyless .env management powered by GitHub</Text>
        {repoStatus.repo && (
          <Text color="gray.600">
            Repository: {repoStatus.repo.owner}/{repoStatus.repo.repo}
          </Text>
        )}
        <Button
          colorScheme="blue"
          size="lg"
          onClick={async () => {
            const { redirect_url } = await api.auth.login();
            window.location.href = redirect_url;
          }}
        >
          Connect with GitHub
        </Button>
      </VStack>
    );
  }

  return (
    <VStack gap={6} align="stretch">
      <Box>
        <Heading size="md">Hello, {status.username}!</Heading>
        {status.repo && (
          <Text color="gray.600">
            Repository: {status.repo.owner}/{status.repo.name}
          </Text>
        )}
      </Box>
    </VStack>
  );
}
