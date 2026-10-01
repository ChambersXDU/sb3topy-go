# v1.2.1 validation

Validated on 2026-10-01 with Python 3.9.6 and pygame 2.6.1 on macOS arm64. This patch keeps the existing CLI and reversible workspace format.

## Changes

`control_wait` now converts its duration with `tonum` and clamps negative durations to zero. `motion_movesteps` now converts its step input with `tonum`. Both fixes apply to literal text, variables, and custom-procedure arguments without changing the canonical Scratch JSON or the stored variable values.

These changes follow the numeric-input handling in Scratch's [wait implementation](https://github.com/scratchfoundation/scratch-vm/blob/b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5/src/blocks/scratch3_control.js#L105-L114) and [move implementation](https://github.com/scratchfoundation/scratch-vm/blob/b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5/src/blocks/scratch3_motion.js#L60-L66).

Workspace mismatch diagnostics now mention converter upgrades as well as Python-only edits. Release builds also run the Python runtime unit tests before publishing binaries.

## Automated regression tests

Run from the repository root:

```sh
go test ./...
python3 -B -m unittest discover -s tests -v
go vet ./...
go build ./...
```

`TestTranspiledWaitAndMoveBlocksExecute` executes generated Python for numeric literals, text numbers, whitespace-padded numbers, negative numbers, empty/non-numeric strings, variables, and procedure arguments. It checks the numeric values actually passed to the target methods, including the difference between negative waits (zero) and negative movement (preserved).

`TestSyncUpgradesWaitCodeWithoutChangingScratchJSON` simulates a workspace containing the previous generator's wait code. It checks the upgrade diagnostic, sync, verify, and repacking with byte-identical canonical project.json.

## SB3 corpus checks

The local corpus contains all 34 SB3 fixtures in scratch-vm at revision `b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5`, plus four local projects: two gun-game versions, a desktop-system project, and a car project. Local projects are not committed or uploaded.

[Official fixtures](https://github.com/scratchfoundation/scratch-vm/tree/b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5/test/fixtures) cover comments, variable names, lists, monitors, script order, extensions, and deliberately missing or damaged resources. In total, the corpus contains 2,228 block entries and 111 distinct opcodes; 22 projects have nonempty block maps. These are coverage counts, not claims that all opcodes are fully implemented.

| Check | Result |
| --- | --- |
| Original input files unchanged | 38/38 |
| Conversion | 34/38 |
| Python syntax compilation and import | 34/34 converted projects |
| Verify and unchanged reverse conversion | 32/34 converted projects |
| Archive entry names and uncompressed content unchanged | 32/32 successful reverse conversions |
| Unchanged sync produces stable Python | 32/32 eligible projects |
| One numeric input edit, sync, verify, and repack | 10/10 eligible projects; other archive entries unchanged |
| Headless runtime initialization | 31/34 converted projects |
| Green-flag execution and drawing for up to 10 seconds | 10/10 eligible projects, with no captured task errors |

The published v1.2.0 executable fails green-flag execution in both official script-order fixtures and both gun-game versions because text-form durations reach asyncio.sleep unchanged. After fixing wait coercion, a longer run exposed text-form movement speed in enemy clones. With both fixes, all ten eligible projects pass the ten-second check. Both released v1.2.0 order fixtures fail even though their syntax, import, and unchanged round-trip checks pass.

Runtime checks use pygame's dummy video/audio drivers. They capture logged task errors and asyncio exception-handler reports, rather than treating process exit alone as success. Each green-flag test starts from a fresh conversion of the original SB3, not from the intentionally edited workspace. A project may stop its runtime before the time limit. Keyboard/mouse interaction, hardware extensions, complete gameplay, and differential behavior against Scratch VM are not covered.

## Remaining boundaries

Three fixtures have project.json inside a containing folder instead of at the ZIP root and are rejected. The Dolphin 3D fixture is rejected because a procedure argument shadow names a parent that does not reference it; loader compatibility needs further investigation before changing graph validation. Two deliberate missing-resource fixtures correctly fail verify/repacking without creating an output. One damaged-audio fixture passes preservation checks but fails pygame decoding. None of these cases was silently normalized or given replacement assets.

The desktop-system project reports 12 explicit unsupported-block issues involving text-extension and drag-mode blocks. Passing preservation and a brief runtime check does not prove those features are implemented.

## Existing workspaces

Preserve any manual Python changes and apply reversible changes to `.sb3topy/project.json` before running the new executable:

```sh
sb3topy sync work/game
sb3topy verify work/game
sb3topy to-sb3 work/game repaired.sb3
```

`sync` overwrites generated project.py. The upgrade itself does not change Scratch JSON or assets. Final repaired projects still need testing in Scratch.
