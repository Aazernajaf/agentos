"""Filesystem contract tests; no provider, inference or third-party package."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('skill_packages', Path(__file__).resolve().parents[1] / 'check_agent_skills.py')
packages = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packages)


class SkillPackageTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.folder = self.root / '.agents/skills/agentos-example'
        self.folder.mkdir(parents=True)
        (self.folder / 'SKILL.md').write_text('---\nname: agentos-example\ndescription: Synthetic workflow\n---\n[Own SOP](SOP.md)\n')
        (self.folder / 'SOP.md').write_text('# Synthetic SOP\n')
        self.alias = self.root / '.claude/skills/agentos-example'
        self.alias.parent.mkdir(parents=True)
        self.alias.symlink_to('../../.agents/skills/agentos-example', target_is_directory=True)

    def test_valid_package(self):
        self.assertEqual(packages.check(self.root)[1], [])

    def test_missing_own_sop_is_rejected(self):
        (self.folder / 'SOP.md').unlink()
        self.assertTrue(packages.check(self.root)[1])

    def test_sop_cannot_be_a_shared_or_external_symlink(self):
        (self.folder / 'SOP.md').unlink()
        target = self.root / 'shared.md'
        target.write_text('# Another SOP\n')
        (self.folder / 'SOP.md').symlink_to(target)
        self.assertTrue(packages.check(self.root)[1])

    def test_alias_cannot_escape_to_other_skill(self):
        other = self.root / 'unrelated'
        other.mkdir()
        self.alias.unlink()
        self.alias.symlink_to(other)
        self.assertTrue(packages.check(self.root)[1])

    def test_own_sop_must_be_reachable_from_entry_point(self):
        (self.folder / 'SKILL.md').write_text('# No workflow reference\n')
        self.assertTrue(packages.check(self.root)[1])

    def test_unknown_native_alias_is_rejected(self):
        (self.alias.parent / 'unexpected').symlink_to(self.folder)
        self.assertTrue(packages.check(self.root)[1])

    def test_missing_native_alias_is_rejected(self):
        self.alias.unlink()
        self.assertTrue(packages.check(self.root)[1])


if __name__ == '__main__':
    unittest.main()
