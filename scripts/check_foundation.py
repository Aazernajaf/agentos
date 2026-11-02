#!/usr/bin/env python3
"""Check foundation Markdown links, whitespace, and embedded JSON; stdlib only."""
import json
import os
import re
import sys
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[1]
REQUIRED = (
    'README.md', 'AGENTS.md', 'CLAUDE.md', 'CONTRIBUTING.md', 'SECURITY.md',
    'CODE_OF_CONDUCT.md', 'docs/product/strategy.md',
    'docs/architecture/overview.md', 'docs/architecture/adapters.md',
    'docs/architecture/events.md', 'docs/security/threat-model.md',
    'docs/adr/README.md', 'docs/handoff/phase-0.md',
)


def check(root):
    errors = []
    for name in REQUIRED:
        if not (root / name).is_file():
            errors.append(f'{name}: required document missing')
    paths = []
    ignored = {'.git', 'node_modules', 'dist', 'build', 'coverage', '__pycache__', 'work'}
    for directory, children, names in os.walk(root):
        children[:] = [name for name in children if name not in ignored and not name.startswith('._')]
        paths.extend(Path(directory) / name for name in names
                     if name.endswith('.md') and not name.startswith('._'))
    paths.sort()
    for path in paths:
        label = path.relative_to(root)
        content = path.read_text(encoding='utf-8')
        if not content.endswith('\n'):
            errors.append(f'{label}: missing final newline')
        for number, line in enumerate(content.splitlines(), 1):
            if line.rstrip() != line:
                errors.append(f'{label}:{number}: trailing whitespace')
        if len(re.findall(r'^```', content, re.M)) % 2:
            errors.append(f'{label}: unclosed code fence')
        for target in re.findall(r'\[[^\]]*\]\(([^\s)]+)\)', content):
            if re.match(r'^[a-zA-Z][a-zA-Z0-9+.-]*:', target) or target.startswith('#'):
                continue
            target = unquote(target.split('#', 1)[0])
            resolved = (path.parent / target).resolve()
            if root.resolve() not in (resolved, *resolved.parents):
                errors.append(f'{label}: link escapes repository: {target}')
            elif not resolved.exists():
                errors.append(f'{label}: broken local link: {target}')
        for snippet in re.findall(r'^```json\n(.*?)^```', content, re.M | re.S):
            try:
                json.loads(snippet)
            except json.JSONDecodeError as exc:
                errors.append(f'{label}: invalid JSON example: {exc}')
    return paths, errors


if __name__ == '__main__':
    documents, failures = check(ROOT)
    for failure in failures:
        print(failure, file=sys.stderr)
    if failures:
        sys.exit(1)
    print(f'PASS: {len(documents)} Markdown files; required docs, local file links, '
          'whitespace, code fences, and JSON examples')
