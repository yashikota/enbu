import { createRootRoute, Outlet } from "@tanstack/react-router";
import { Box, Container, Heading, Flex, NativeSelect } from "@chakra-ui/react";
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
      </Flex>
      <Container maxW="container.lg" py={8}>
        <Outlet />
      </Container>
    </Box>
  );
}
