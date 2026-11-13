#!/usr/bin/env python3
"""Select the changelog section for a release tag."""

from pathlib import Path
import re
import sys


ROOT = Path(__file__).resolve().parent.parent
TAG = re.compile(
    r"^v(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)"
    r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?"
    r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)


def release_notes(changelog, tag):
    if not TAG.fullmatch(tag):
        raise ValueError(f"Invalid release tag: {tag}")
    header = re.compile(rf"^## {re.escape(tag[1:])} - \d{{4}}-\d{{2}}-\d{{2}}$")
    lines = changelog.splitlines()
    matches = [index for index, line in enumerate(lines) if header.fullmatch(line)]
    if len(matches) != 1:
        raise ValueError(f"Expected exactly one dated changelog section for {tag}")
    start = matches[0] + 1
    end = next(
        (index for index in range(start, len(lines)) if re.match(r"^##(?:\s|$)", lines[index])),
        len(lines),
    )
    notes = "\n".join(lines[start:end]).strip()
    if not notes:
        raise ValueError(f"Changelog section for {tag} is empty")
    return notes + "\n"


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("Usage: python3 scripts/release-notes.py TAG")
    try:
        changelog = (ROOT / "CHANGELOG.md").read_text(encoding="utf-8")
        sys.stdout.write(release_notes(changelog, sys.argv[1]))
    except (OSError, ValueError) as error:
        sys.exit(f"Cannot prepare release notes: {error}")
