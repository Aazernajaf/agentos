#!/usr/bin/env python3
"""Validate repository skill/SOP ownership and canonical Claude alias integrity."""
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]


def check(root):
    root = root.resolve()
    base = root / '.agents/skills'
    aliases = root / '.claude/skills'
    errors = []
    folders = sorted(p for p in base.iterdir() if not p.name.startswith('._')) if base.is_dir() else []
    if not folders:
        return [], ['No repository skills found']
    for folder in folders:
        name = folder.name
        label = '.agents/skills/' + name
        if (not re.fullmatch(r'[a-z0-9]+(?:-[a-z0-9]+)*', name) or len(name) > 64
                or not folder.is_dir() or folder.is_symlink()):
            errors.append(label + ': invalid canonical folder')
            continue
        for filename in ('SKILL.md', 'SOP.md'):
            path = folder / filename
            if not path.is_file() or path.is_symlink() or path.stat().st_size == 0:
                errors.append(label + ': missing/empty/noncanonical ' + filename)
        skill = folder / 'SKILL.md'
        if skill.is_file() and not skill.is_symlink():
            targets = re.findall(r'\[[^\]]*\]\(([^\s)]+)\)', skill.read_text())
            if not any((folder / target).resolve() == (folder / 'SOP.md').resolve()
                       for target in targets if '://' not in target):
                errors.append(label + ': entry point must link its own SOP')
        alias = aliases / name
        if not alias.is_symlink() or alias.resolve() != folder.resolve():
            errors.append('.claude/skills/' + name + ': alias must target canonical skill')
    expected = {folder.name for folder in folders}
    if aliases.is_dir():
        for alias in aliases.iterdir():
            if not alias.name.startswith('._') and alias.name not in expected:
                errors.append('.claude/skills/' + alias.name + ': unregistered alias')
    return folders, errors


if __name__ == '__main__':
    folders, failures = check(ROOT)
    if failures:
        for failure in failures:
            print(failure, file=sys.stderr)
        sys.exit(1)
    print(f'PASS: {len(folders)} canonical skills, own SOP links and Claude aliases; not behavioral enforcement')
