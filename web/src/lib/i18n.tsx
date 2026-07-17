import { createContext, useContext, useMemo, useState, type ReactNode } from "react";

const en = {
  app: {
    name: "enbu",
    language: "Language",
  },
  repo: {
    selectTitle: "Select repository",
    selectDescription: "Choose the local GitHub repository that enbu should manage.",
    browse: "Browse",
    pathPlaceholder: "C:\\Users\\you\\src\\your-repo",
    continue: "Continue",
    current: "Repository: {owner}/{repo}",
  },
  init: {
    title: "Set up enbu",
    description: "This repository hasn't been set up with enbu yet.",
    action: "Set up",
    success: "Initialized {environment}.",
    gitTitle: "Initialize Git",
    gitDescription: "This folder is not a Git repository yet.",
    gitAction: "Initialize Git",
    remoteTitle: "Create GitHub repository",
    repositoryOwner: "Owner",
    personalAccount: "Personal account",
    organization: "Organization",
    repositoryName: "Repository name",
    privateRepository: "Private repository",
    createRemote: "Create repository",
  },
  dashboard: {
    secrets: "Secrets",
    environments: "Environments",
    empty: "No secrets yet.",
    key: "Key",
    value: "Value",
    add: "Add secret",
    save: "Save",
    delete: "Delete",
    deleteSecretConfirm: "Delete {key}?",
    pull: "Pull",
    sync: "Sync",
    pullDescription: "Pull secrets from GHCR.",
    syncDescription: "Sync the member list.",
    newEnvironment: "New environment",
    createEnvironment: "Create",
    addEnvironment: "Add environment...",
    createEnvironmentTitle: "Create environment",
    environmentName: "Environment name",
    secretHeadingSuffix: "secrets",
    copyKey: "Copy key",
    copyValue: "Copy value",
    keyCopied: "Key copied",
    valueCopied: "Value copied",
    currentEnvironment: "Current environment",
    current: "Current",
    activeEnvironment: "Active environment",
    switchEnvironment: "Click to switch",
    environmentSecrets: "{environment} secrets",
  },
  auth: {
    welcome: "Sign in to GitHub",
    tagline: "Log in to GitHub.",
    connect: "Connect with GitHub",
    waiting: "Waiting for GitHub authorization...",
    userCode: "Enter this code on GitHub",
    copied: "Copied",
    copyFailed: "Copy failed",
    browserOpened: "GitHub opened in your browser.",
    browserNotOpened: "Open GitHub and enter the code.",
    copyCode: "Copy code",
    openGitHub: "Open GitHub",
    cancel: "Cancel",
    tryAgain: "Try again",
    expiresIn: "Expires in {seconds}s",
    denied: "Authorization was cancelled.",
    expired: "The code expired.",
    error: "Authorization failed.",
    hello: "Hello, {username}!",
    signedIn: "Signed in",
    signedOut: "Not signed in",
    checkFailed: "Auth status unavailable",
    logout: "Log out",
    authorizeTitle: "GitHub Login",
    autoRedirect: "Opening GitHub in {seconds}s...",
    codeInstruction: "Copy this code and paste it on GitHub.",
  },
  common: {
    loading: "Loading...",
    error: "Error",
  },
  sidebar: {
    repositories: "Repositories",
    addRepository: "Add repository",
    noRepositories: "No repositories yet.",
    remove: "Remove",
    removeConfirm: "Remove {repository} from enbu?",
    active: "active",
  },
  recipients: {
    title: "Recipients",
    members: "Members",
    member: "Member",
    empty: "No recipients found.",
    username: "Username",
    fingerprint: "Fingerprint",
  },
  config: {
    title: "enbu.toml",
    settings: "Settings",
    edit: "Edit",
    save: "Save",
    cancel: "Cancel",
    saveError: "Failed to save.",
    invalidToml: "Fix the TOML errors before switching to the GUI editor.",
    guiView: "GUI editor",
    codeView: "Code editor",
    general: "General",
    defaultEnvironment: "Default environment",
    outputFileNames: "Output file names",
  },
};

type Messages = typeof en;

const ja: Messages = {
  app: {
    name: "enbu",
    language: "言語",
  },
  repo: {
    selectTitle: "リポジトリを選択",
    selectDescription: "enbuで管理するローカルのGitHubリポジトリを選択してください。",
    browse: "参照",
    pathPlaceholder: "C:\\Users\\you\\src\\your-repo",
    continue: "続行",
    current: "リポジトリ: {owner}/{repo}",
  },
  init: {
    title: "enbuのセットアップ",
    description: "このリポジトリはまだenbuのセットアップが完了していません。",
    action: "セットアップ",
    success: "{environment} を初期化しました。",
    gitTitle: "Gitを初期化",
    gitDescription: "このフォルダはまだGitリポジトリではありません。",
    gitAction: "Gitを初期化",
    remoteTitle: "GitHubリポジトリを作成",
    repositoryOwner: "作成先アカウント",
    personalAccount: "個人アカウント",
    organization: "Organization",
    repositoryName: "リポジトリ名",
    privateRepository: "非公開リポジトリ",
    createRemote: "リポジトリを作成",
  },
  dashboard: {
    secrets: "シークレット",
    environments: "環境",
    empty: "シークレットはまだありません。",
    key: "キー",
    value: "値",
    add: "シークレットを追加",
    save: "保存",
    delete: "削除",
    deleteSecretConfirm: "{key} を削除しますか？",
    pull: "Pull",
    sync: "Sync",
    pullDescription: "GHCRからシークレットを取得します。",
    syncDescription: "メンバー一覧を同期します。",
    newEnvironment: "新しい環境",
    createEnvironment: "作成",
    addEnvironment: "環境を追加…",
    createEnvironmentTitle: "環境を作成",
    environmentName: "環境名",
    secretHeadingSuffix: "のシークレット",
    copyKey: "キーをコピー",
    copyValue: "値をコピー",
    keyCopied: "キーをコピー済み",
    valueCopied: "値をコピー済み",
    currentEnvironment: "現在の環境",
    current: "現在",
    activeEnvironment: "現在使用中の環境",
    switchEnvironment: "クリックして切り替え",
    environmentSecrets: "{environment} のシークレット",
  },
  auth: {
    welcome: "GitHubにサインイン",
    tagline: "GitHubにログインしてください。",
    connect: "GitHubで接続",
    waiting: "GitHubでの承認を待っています...",
    userCode: "GitHubでこのコードを入力してください",
    copied: "コピー済み",
    copyFailed: "コピーできませんでした",
    browserOpened: "GitHubをブラウザで開きました。",
    browserNotOpened: "GitHubを開いてコードを入力してください。",
    copyCode: "コードをコピー",
    openGitHub: "GitHubを開く",
    cancel: "キャンセル",
    tryAgain: "再試行",
    expiresIn: "有効期限まで {seconds} 秒",
    denied: "認証がキャンセルされました。",
    expired: "コードの有効期限が切れました。",
    error: "認証に失敗しました。",
    hello: "{username} さん、こんにちは！",
    signedIn: "ログイン中",
    signedOut: "未ログイン",
    checkFailed: "認証状態を確認できません",
    logout: "ログアウト",
    authorizeTitle: "GitHub 認証",
    autoRedirect: "{seconds}秒後にGitHubを開きます...",
    codeInstruction: "このコードをコピーして、GitHubに貼り付けてください。",
  },
  common: {
    loading: "読み込み中...",
    error: "エラー",
  },
  sidebar: {
    repositories: "リポジトリ",
    addRepository: "リポジトリを追加",
    noRepositories: "まだリポジトリがありません。",
    remove: "削除",
    removeConfirm: "{repository} をenbuから削除しますか？",
    active: "使用中",
  },
  recipients: {
    title: "受信者一覧",
    members: "メンバー",
    member: "Member",
    empty: "受信者が見つかりません。",
    username: "ユーザー名",
    fingerprint: "フィンガープリント",
  },
  config: {
    title: "enbu.toml",
    settings: "設定",
    edit: "編集",
    save: "保存",
    cancel: "キャンセル",
    saveError: "保存に失敗しました。",
    invalidToml: "TOMLのエラーを修正してからGUI編集へ切り替えてください。",
    guiView: "GUIで編集",
    codeView: "コードで編集",
    general: "一般",
    defaultEnvironment: "デフォルト環境",
    outputFileNames: "出力ファイル名",
  },
};

const dictionaries = { en, ja };
export type Locale = keyof typeof dictionaries;

type TranslationParams = Record<string, string | number>;
type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: MessageKey, params?: TranslationParams) => string;
};

type NestedKeys<T> = {
  [K in keyof T]: T[K] extends string ? K & string : `${K & string}.${NestedKeys<T[K]>}`;
}[keyof T];

export type MessageKey = NestedKeys<Messages>;

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => detectLocale());
  const value = useMemo<I18nContextValue>(
    () => ({
      locale,
      setLocale: (next) => {
        localStorage.setItem("enbu_locale", next);
        setLocaleState(next);
      },
      t: (key, params) => translate(locale, key, params),
    }),
    [locale],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return ctx;
}

export function detectLocale(): Locale {
  const saved = typeof localStorage !== "undefined" ? localStorage.getItem("enbu_locale") : null;
  if (isLocale(saved)) {
    return saved;
  }
  const language = navigator.language.toLowerCase();
  if (language.startsWith("ja")) {
    return "ja";
  }
  return "en";
}

export function translate(locale: Locale, key: MessageKey, params: TranslationParams = {}) {
  const template = getMessage(dictionaries[locale], key) ?? getMessage(en, key) ?? key;
  return Object.entries(params).reduce(
    (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)),
    template,
  );
}

function getMessage(messages: Messages, key: MessageKey): string | undefined {
  return key.split(".").reduce<unknown>((value, part) => {
    if (value && typeof value === "object" && part in value) {
      return (value as Record<string, unknown>)[part];
    }
    return undefined;
  }, messages) as string | undefined;
}

function isLocale(value: string | null): value is Locale {
  return value === "en" || value === "ja";
}
