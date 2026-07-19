import type { ComponentProps } from "react";
import { Flex } from "styled-system/jsx";
import { Text } from "./ui";
import { NativeSelect } from "./ui/native-select";
import { useI18n, type Locale } from "../lib/i18n";

type LanguageSelectorProps = ComponentProps<typeof Flex>;

export function LanguageSelector(props: LanguageSelectorProps) {
  const { locale, setLocale, t } = useI18n();

  return (
    <Flex alignItems="center" gap="2" {...props}>
      <Text fontSize="sm" color="fg.muted">
        {t("app.language")}
      </Text>
      <NativeSelect
        value={locale}
        onChange={(event) => setLocale(event.target.value as Locale)}
        style={{ width: "118px" }}
      >
        <option value="en">English</option>
        <option value="ja">日本語</option>
      </NativeSelect>
    </Flex>
  );
}
