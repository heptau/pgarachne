#!/usr/bin/env python3
"""
check_i18n.py

Validates that all i18n YAML files in docs-src/i18n/ contain exactly
the same translation keys as the reference file (en.yaml).

Exit codes:
   0 — all files are complete
   1 — one or more files have missing or extra keys
"""

import sys
import yaml
from pathlib import Path


I18N_DIR = Path(__file__).parent.parent / "docs-src" / "i18n"
REFERENCE_LANG = "en"


def load_keys(path: Path) -> set[str]:
   """Load a YAML translation file and return the set of key IDs."""
   with path.open(encoding="utf-8") as f:
      entries = yaml.safe_load(f)
   if not isinstance(entries, list):
      raise ValueError(f"{path}: expected a YAML list, got {type(entries).__name__}")
   return {entry["id"] for entry in entries}


def main() -> int:
   reference_file = I18N_DIR / f"{REFERENCE_LANG}.yaml"
   if not reference_file.exists():
      print(f"ERROR: reference file not found: {reference_file}", file=sys.stderr)
      return 1

   reference_keys = load_keys(reference_file)
   print(f"Reference ({REFERENCE_LANG}.yaml): {len(reference_keys)} keys")

   all_ok = True

   for yaml_file in sorted(I18N_DIR.glob("*.yaml")):
      lang = yaml_file.stem
      if lang == REFERENCE_LANG:
         continue

      try:
         keys = load_keys(yaml_file)
      except Exception as exc:
         print(f"  [{lang}] ERROR reading file: {exc}")
         all_ok = False
         continue

      missing = reference_keys - keys
      extra = keys - reference_keys

      if missing or extra:
         all_ok = False
         print(f"  [{lang}] FAIL")
         if missing:
            for key in sorted(missing):
               print(f"         MISSING: {key}")
         if extra:
            for key in sorted(extra):
               print(f"         EXTRA:   {key}")
      else:
         print(f"  [{lang}] OK ({len(keys)} keys)")

   if all_ok:
      print("\nAll translation files are complete.")
      return 0
   else:
      print("\nSome translation files have issues. See above.", file=sys.stderr)
      return 1


if __name__ == "__main__":
   sys.exit(main())
