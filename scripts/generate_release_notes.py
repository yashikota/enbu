#!/usr/bin/env python3
"""Generate GitHub release notes with a table of downloadable binaries."""

from __future__ import annotations

import argparse
import os
import re
import sys
from collections.abc import Mapping
from pathlib import Path
from urllib.parse import quote

AssetKey = tuple[str, str]

OS_LABELS = {
    "darwin": "macOS",
    "linux": "Linux",
    "windows": "Windows",
}
ARCH_LABELS = {
    "amd64": "x64",
    "arm64": "arm64",
}
OS_ORDER = {"darwin": 0, "linux": 1, "windows": 2}
ARCH_ORDER = {"amd64": 0, "arm64": 1}


class ReleaseNotesError(ValueError):
    """Raised when release notes cannot be generated safely."""


def collect_assets(
    assets_dir: Path, tag: str
) -> tuple[dict[AssetKey, str], dict[AssetKey, str]]:
    """Return CLI and desktop release assets keyed by OS and architecture."""
    if not assets_dir.is_dir():
        raise ReleaseNotesError(f"release assets directory not found: {assets_dir}")

    escaped_tag = re.escape(tag)
    cli_pattern = re.compile(rf"^enbu_{escaped_tag}_([^_]+)_([^_.]+)\.(?:tar\.gz|zip)$")
    desktop_pattern = re.compile(
        rf"^enbu-desktop_{escaped_tag}_([^_]+)_([^_.]+)\.(?:tar\.gz|zip|dmg)$"
    )
    cli_assets: dict[AssetKey, str] = {}
    desktop_assets: dict[AssetKey, str] = {}

    for path in sorted(assets_dir.iterdir()):
        if not path.is_file():
            continue

        match = desktop_pattern.fullmatch(path.name)
        destination = desktop_assets
        kind = "desktop"
        if match is None:
            match = cli_pattern.fullmatch(path.name)
            destination = cli_assets
            kind = "CLI"
        if match is None:
            continue

        key = match.group(1), match.group(2)
        if existing := destination.get(key):
            raise ReleaseNotesError(
                f"multiple {kind} assets found for {key[0]}/{key[1]}: "
                f"{existing}, {path.name}"
            )
        destination[key] = path.name

    if not cli_assets and not desktop_assets:
        raise ReleaseNotesError(
            f"no CLI or desktop release assets found in {assets_dir} for tag {tag}"
        )

    return cli_assets, desktop_assets


def asset_url(repository: str, tag: str, name: str) -> str:
    """Build a GitHub download URL with safely encoded path components."""
    repository_path = quote(repository, safe="/")
    return (
        f"https://github.com/{repository_path}/releases/download/"
        f"{quote(tag, safe='')}/{quote(name, safe='')}"
    )


def asset_link(repository: str, tag: str, name: str | None) -> str:
    if name is None:
        return "—"
    return f"[`{name}`]({asset_url(repository, tag, name)})"


def asset_sort_key(key: AssetKey) -> tuple[int, str, int, str]:
    os_name, arch = key
    return (
        OS_ORDER.get(os_name, len(OS_ORDER)),
        os_name,
        ARCH_ORDER.get(arch, len(ARCH_ORDER)),
        arch,
    )


def render_release_notes(
    repository: str,
    tag: str,
    cli_assets: Mapping[AssetKey, str],
    desktop_assets: Mapping[AssetKey, str],
    generated_notes: str = "",
) -> str:
    """Render the download table followed by GitHub-generated notes."""
    keys = sorted(cli_assets.keys() | desktop_assets.keys(), key=asset_sort_key)
    lines = [
        "## Downloads",
        "",
        "| OS | Arch | CLI | Desktop |",
        "| --- | --- | --- | --- |",
    ]

    for os_name, arch in keys:
        key = os_name, arch
        lines.append(
            f"| {OS_LABELS.get(os_name, os_name)} | {ARCH_LABELS.get(arch, arch)} | {asset_link(repository, tag, cli_assets.get(key))} | {asset_link(repository, tag, desktop_assets.get(key))} |"
        )

    if generated_notes.strip():
        lines.extend(("", generated_notes.strip()))

    return "\n".join(lines) + "\n"


def env_default(name: str) -> str | None:
    value = os.environ.get(name)
    return value if value else None


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--assets-dir",
        type=Path,
        default=Path(os.environ.get("RELEASE_ASSETS_DIR", "release-assets")),
    )
    parser.add_argument("--repository", default=env_default("GITHUB_REPOSITORY"))
    parser.add_argument("--tag", default=env_default("GITHUB_REF_NAME"))
    parser.add_argument(
        "--generated-notes",
        type=Path,
        default=(
            Path(value)
            if (value := env_default("GENERATED_NOTES_FILE")) is not None
            else None
        ),
    )
    args = parser.parse_args(argv)
    if args.repository is None:
        parser.error("--repository or GITHUB_REPOSITORY is required")
    if args.tag is None:
        parser.error("--tag or GITHUB_REF_NAME is required")
    return args


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        cli_assets, desktop_assets = collect_assets(args.assets_dir, args.tag)
        generated_notes = (
            args.generated_notes.read_text(encoding="utf-8")
            if args.generated_notes is not None
            else ""
        )
        sys.stdout.write(
            render_release_notes(
                args.repository,
                args.tag,
                cli_assets,
                desktop_assets,
                generated_notes,
            )
        )
    except (OSError, ReleaseNotesError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
