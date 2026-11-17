#!/usr/bin/env python3
"""Compose verified before/after captures without altering their image contents."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re

from PIL import Image, ImageDraw, ImageFont


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--evidence', type=Path, required=True)
    parser.add_argument('--before-dir', type=Path)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    evidence = args.evidence.resolve()
    parent = evidence.parent
    before_dir = (args.before_dir or parent / 'baseline/paired-clean').resolve()
    def ui_version(path):
        match = re.search(r'"ui_version"\s*:\s*"([^"]+)"', path.read_text())
        if not match:
            raise ValueError('UI build information is missing: ' + str(path))
        return match.group(1)
    before_version = ui_version(parent / 'baseline/frozen/plugin/ui/index.html')
    after_version = ui_version(root / 'plugin/ui/index.html')
    before = json.loads((before_dir / 'ui-results.json').read_text())
    after = json.loads((evidence / 'ui-results.json').read_text())
    assert before['result'] == after['result'] == 'passed'
    pairs = []
    hashes = {}
    for old in sorted(before_dir.glob('paired-*.png')):
        new = evidence / old.name
        assert before['capture_fixture_sha256'][old.name] == after['capture_fixture_sha256'][new.name]
        assert digest(old) == before['screenshot_sha256'][old.name]
        assert digest(new) == after['screenshot_sha256'][new.name]
        with Image.open(old) as old_image, Image.open(new) as new_image:
            assert old_image.size == new_image.size
            dimensions = list(old_image.size)
        for path in (old, new):
            hashes[str(path.relative_to(root))] = digest(path)
        pairs.append({'name': old.stem, 'before': str(old.relative_to(root)),
                      'after': str(new.relative_to(root)), 'dimensions': dimensions,
                      'data': 'Same explicit offline fake-management task fixtures; candy answers are 21/unknown/manual, pause scenario retains operator disable.'})
    assert len(pairs) == 8
    font_path = '/System/Library/Fonts/Supplemental/Arial.ttf'
    font = ImageFont.truetype(font_path, 22) if Path(font_path).exists() else ImageFont.load_default()
    for name, capture in (('comparison-desktop.png', 'paired-setup-desktop.png'),
                          ('comparison-mobile.png', 'paired-evidence-mobile.png')):
        with Image.open(before_dir / capture).convert('RGB') as old, Image.open(evidence / capture).convert('RGB') as new:
            gutter, top = 24, 56
            canvas = Image.new('RGB', (old.width + new.width + gutter * 3, max(old.height, new.height) + top + gutter), '#eeeeea')
            draw = ImageDraw.Draw(canvas)
            draw.text((gutter, 16), 'Before: UI ' + before_version, font=font, fill='#2d2a26')
            draw.text((old.width + gutter * 2, 16), 'After: UI ' + after_version, font=font, fill='#2d2a26')
            canvas.paste(old, (gutter, top))
            canvas.paste(new, (old.width + gutter * 2, top))
            output = parent / name
            canvas.save(output)
            hashes[str(output.relative_to(root))] = digest(output)
    report = {'result': 'passed', 'created_at': datetime.now(timezone.utc).isoformat(),
              'source': f'Verified frozen baseline {before_version} and final {after_version} browser captures. Collages add labels and spacing only; no crop or image retouching.',
              'pairs': pairs, 'sha256': hashes}
    (parent / 'comparison-provenance.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'result': 'passed', 'pairs': len(pairs), 'output': str(parent)}, indent=2))


if __name__ == '__main__':
    main()
