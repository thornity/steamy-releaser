import importlib.util
from pathlib import Path
import sys
import unittest


sys.dont_write_bytecode = True
SCRIPT = Path(__file__).with_name("release-notes.py")
SPEC = importlib.util.spec_from_file_location("release_notes", SCRIPT)
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


class ReleaseNotesTests(unittest.TestCase):
    def test_selects_only_the_requested_dated_section(self):
        changelog = """# Changelog

## 0.2.1 - 2026-11-13

### Fixed

- Mobile approval.

## 0.2.0 - 2026-11-10

- Previous release.
"""
        self.assertEqual(
            release.release_notes(changelog, "v0.2.1"),
            "### Fixed\n\n- Mobile approval.\n",
        )

    def test_rejects_missing_duplicate_empty_and_invalid_sections(self):
        with self.assertRaisesRegex(ValueError, "exactly one"):
            release.release_notes("## 0.2.0 - 2026-11-10\n\n- Old\n", "v0.2.1")
        with self.assertRaisesRegex(ValueError, "exactly one"):
            release.release_notes(
                "## 0.2.1 - 2026-11-13\n\n- One\n\n"
                "## 0.2.1 - 2026-11-14\n\n- Two\n",
                "v0.2.1",
            )
        with self.assertRaisesRegex(ValueError, "empty"):
            release.release_notes("## 0.2.1 - 2026-11-13\n\n## Unreleased\n", "v0.2.1")
        with self.assertRaisesRegex(ValueError, "Invalid release tag"):
            release.release_notes("## 0.2.1 - 2026-11-13\n\n- Note\n", "v0.2.1/other")


if __name__ == "__main__":
    unittest.main()
