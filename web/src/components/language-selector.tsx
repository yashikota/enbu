import type { ComponentProps } from "react";
import { Flex, styled } from "styled-system/jsx";
import { NativeSelect } from "./ui/native-select";
import { useI18n, type Locale } from "../lib/i18n";

type LanguageSelectorProps = ComponentProps<typeof Flex>;

export function LanguageSelector(props: LanguageSelectorProps) {
  const { locale, setLocale, t } = useI18n();

  return (
    <Flex alignItems="center" gap="2" {...props}>
      <styled.label htmlFor="language-select" fontSize="sm" color="fg.muted">
        {t("app.language")}
      </styled.label>
      <NativeSelect
        id="language-select"
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
