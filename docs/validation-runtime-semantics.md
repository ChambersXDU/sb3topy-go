# Runtime semantics and agent diagnostics validation

Validated on 2026-10-01 with Go 1.25.2, Python 3.9.6, and pygame 2.6.1 on macOS arm64. The CLI and reversible workspace format remain unchanged. CI and Release use the Go version from go.mod, Python 3.11, and pygame 2.6.1.

## Changes covered

Inspection distinguishes implemented, partial, unsupported, unmapped, and unavailable translations. Known runtime placeholders and partial behaviors are reported explicitly. These statuses describe the generated translation, not proof of full behavioral equivalence with Scratch.

Unsupported reporter inputs propagate through nested expressions to the dependent stack block. They no longer invent movement defaults, overwrite variables with zero, or silently choose a control-flow branch. Skipped control bodies retain unique unmapped source associations. Threshold hats with unsupported inputs are not activated in Python and receive an unsupported-input issue even when inspected individually.

Timed speech and thought retain their waiting duration; bubbles are still not rendered. Common turn, direction, coordinate, and glide inputs use numeric coercion for text literals, variables, and custom-procedure arguments. Glide interpolation is clamped to avoid overshooting the destination on its final tick.

Repeat counts use ties-toward-positive-infinity rounding. Character indices check bounds before truncation and use UTF-16 units. Boolean operators and conditions recognize Scratch-false strings, including "0" and "false". Text operations share boolean and integral-number formatting; string length uses the same UTF-16 units as character indexing.

Cross-sprite clones are registered under the cloned sprite and placed behind that original, not the creator. They participate in the original sprite's collision checks and can be deleted without leaving a stale global clone entry.

Edge bouncing now reflects away from the nearest stage edge and fences the rendered costume back into the stage. It follows Scratch's nearest-edge selection and minimum reflected component, but uses integer-pixel pygame bounds, so its rendering/fencing precision is explicitly reported as partial.

Timer hats evaluate threshold inputs in the running target context, support variable thresholds, and trigger on a false-to-true transition rather than continuously while above the threshold. Timer transitions during a waiting handler can still be missed, so the hat remains partial. Loudness threshold hats are unsupported.

These semantics follow Scratch's pinned [operators](https://github.com/scratchfoundation/scratch-vm/blob/b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5/src/blocks/scratch3_operators.js), [control blocks](https://github.com/scratchfoundation/scratch-vm/blob/b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5/src/blocks/scratch3_control.js), [motion blocks](https://github.com/scratchfoundation/scratch-vm/blob/b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5/src/blocks/scratch3_motion.js), and [casting rules](https://github.com/scratchfoundation/scratch-vm/blob/b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5/src/util/cast.js). This is source-guided regression testing, not a complete differential test against Scratch VM.

## Legacy procedure compatibility

Scratch's [block container](https://github.com/scratchfoundation/scratch-vm/blob/b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5/src/engine/blocks.js) creates blocks without requiring reciprocal parent links, finds procedure definitions through custom_block, and reads parameter metadata from prototype mutations. The official Dolphin 3D fixture contains legacy prototypes with missing parent links and omitted argument-shadow inputs.

Only an unambiguous legacy shape is reconstructed in the temporary validation view: a shadow prototype with no parent or inputs, a unique top-level definition, and usable argument names/IDs. Parameter shadows must match uniquely. The canonical JSON is never rewritten. Tests reject duplicate definitions, wrong argument names, non-shadow arguments, duplicate shadows, missing definitions, ordinary orphans, and malformed mutation metadata.

The Dolphin 3D fixture now converts, verifies, repacks, initializes, and passes the ten-second green-flag smoke check. All 19 original ZIP entries retain byte-identical uncompressed content before any intentional edit.

## Permanent tests

The Python suite has 23 tests: eight list tests, five operator tests, and ten real-SB3 behavior scenarios. Each behavior scenario creates an authored project and asset, runs CLI conversion, verify, and reverse conversion, compares all archive entries, then runs the actual generated Python/runtime under dummy video/audio drivers. Logged task errors and asyncio exception-handler reports fail the test.

Behavior assertions cover repeat/character results, boolean and text reporters including procedure parameters, timed speech/thought order, numeric motion and procedure inputs, clone positions, contact counted once, green-flag health reset, unsupported reporter propagation, cross-sprite clone collision/layering/deletion, variable timer thresholds with rearming, and edge reflection/fencing at all four edges and a grazing angle. Go tests additionally cover 30 numeric-motion opcode/input combinations, inspection statuses, source mapping, and legacy-graph preservation and rejection cases.

Five newly targeted behavior tests also fail with the published v1.2.1 executable: boolean/text semantics, cross-sprite clone ownership, unsupported reporters inventing values, variable timer thresholds, and edge bouncing. The cross-sprite test uses numeric literals for movement so the baseline reaches the clone-ownership assertion instead of failing first on numeric coercion.

## Corpus regression

The corpus is the same 34 official SB3 fixtures at revision b3266a0cfe5122f20b72ccd738a3dd4dff4fc5a5 plus four local projects used for v1.2.1. Local project contents and logs are not committed or uploaded.

| Check | Result |
| --- | --- |
| Original inputs unchanged | 38/38 |
| Conversion | 35/38 |
| Python syntax and import | 35/35 converted projects |
| Verify and unchanged reverse conversion | 33/35 converted projects |
| Archive names and uncompressed content unchanged | 33/33 successful reverse conversions |
| Unchanged sync produces stable Python | 33/33 eligible projects |
| One numeric input edit, sync, verify, repack | 11/11 eligible projects |
| Headless runtime initialization | 32/35 converted projects |
| Green-flag execution and drawing for up to ten seconds | 11/11 eligible projects; no captured task errors |

Three fixtures with nested project.json are still rejected. Two intentionally missing-resource fixtures fail verify/repacking without producing output. Those two fixtures and one damaged-audio fixture cannot initialize the pygame runtime. Assets are neither replaced nor silently repaired.

This validation does not establish complete gameplay correctness, hardware-extension support, pixel-identical rendering, or full Scratch scheduling equivalence. Edge bouncing uses approximate pixel bounds; bubbles, graphical monitors, and several extensions remain absent or partial and must not be mistaken for defects in a student's Scratch project. Final repaired projects still need testing in Scratch.

## Existing workspaces

Generation changes require sync with the updated executable. Preserve manual Python edits and apply reversible fixes to canonical Scratch JSON before syncing, because sync overwrites project.py. Verify afterward, then repack. Neither the upgrade nor the legacy compatibility handling rewrites Scratch JSON or original assets on its own.
