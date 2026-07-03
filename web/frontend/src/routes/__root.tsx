import { createRootRoute, Outlet } from "@tanstack/react-router";
import { Box, Container, Heading, Flex } from "@chakra-ui/react";

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  return (
    <Box minH="100vh" bg="gray.50">
      <Flex as="header" bg="white" borderBottomWidth="1px" py={3} px={6}>
        <Heading size="md">enbu</Heading>
      </Flex>
      <Container maxW="container.lg" py={8}>
        <Outlet />
      </Container>
    </Box>
  );
}
