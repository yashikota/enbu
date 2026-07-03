import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { Box, Button, Heading, Text, VStack, Spinner } from "@chakra-ui/react";
import { api, type AuthStatus } from "../lib/api";

export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.auth
      .status()
      .then(setStatus)
      .catch(() => setStatus({ authenticated: false }))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <VStack py={20}>
        <Spinner size="xl" />
      </VStack>
    );
  }

  if (!status?.authenticated) {
    return (
      <VStack gap={6} py={20}>
        <Heading size="lg">Welcome to enbu</Heading>
        <Text color="gray.600">
          Keyless .env management powered by GitHub
        </Text>
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
