# Codex task — bare-metal install ISO

Scope: ONLY a new `packaging/iso/` directory, plus one new GitHub Actions workflow file `.github/workflows/build-iso.yml` (separate file, don't touch the existing `ci.yml`). Do not touch `internal/`, `cmd/`, `web/`, or the existing `packaging/install.sh`/`packaging/systemd`/`packaging/nfpm.*` files — those are done and reviewed already.

## Context

FaroOS is an open-source (AGPL-3.0) multi-server control panel. It already installs cleanly on an existing Linux box via `packaging/install.sh` (curl-installer) or `.deb`/`.rpm` packages — that part is done. This task is the last install path: a bootable ISO so someone can take a blank machine, boot it, and end up with FaroOS running with no manual steps.

**Important scope-setting, read this before starting:** we are explicitly NOT building a custom Linux distro/kernel (that's what ZimaOS does, via Buildroot, and it's a multi-month undertaking for a dedicated team — we looked into it and deliberately ruled it out for this project). What we ARE building is an **Ubuntu Server autoinstall ISO**: a stock Ubuntu Server ISO repacked with an `autoinstall` (subiquity/cloud-init) config that answers all the installer's questions non-interactively and, once the base OS is installed, runs a first-boot script that installs Docker and FaroOS (reusing `packaging/install.sh`, already written) as a systemd service. This is the same technique used by e.g. MAAS and countless "unattended Ubuntu install" setups — well-documented, not experimental.

Reference docs to work from (Ubuntu's own autoinstall documentation, which you should fetch/read as part of this task since the exact YAML schema and the ISO-repacking steps matter for correctness):
- https://ubuntu.com/server/docs/install/autoinstall
- https://ubuntu.com/server/docs/install/autoinstall-quickstart
- https://ubuntu.com/server/docs/install/autoinstall-reference

## Task 1: autoinstall config

Create `packaging/iso/autoinstall/user-data` (cloud-init/subiquity autoinstall YAML) and `packaging/iso/autoinstall/meta-data` (can be empty/minimal, autoinstall still wants the file to exist for the nocloud datasource). The user-data should:
- Do a minimal Ubuntu Server install (latest LTS), English locale, US or unspecified keyboard (pick a sane default, document how to change it in the README this task adds).
- Partition the whole first disk automatically (`storage: layout: name: direct` in autoinstall terms) — this is a "wipes the disk" installer by nature, same as ZimaOS's own installer; make sure this is loudly documented, not a silent surprise.
- Create a first user account. Since we can't know the real user's desired username/password at ISO-build time, either (a) prompt for them at ISO-build time and bake in a hashed password (documented clearly as "rebuild the ISO to change this, or change it after first boot"), or (b) set a clearly-labeled default (e.g. user `faroos-admin`, a random generated password printed at the end of the build script AND written to a `CREDENTIALS.txt` next to the built ISO) — pick whichever is more consistent with autoinstall's actual capabilities once you've read the reference docs, and explain the choice in the README this task adds.
- Install `docker.io` and `curl` as packages during the install (`packages:` section) so Docker is ready immediately.
- Use autoinstall's `late-commands` (runs at the end of install, chrooted into the target system) to fetch and run `packaging/install.sh` from this repo's GitHub raw URL with `-s -- server` (deploying the central panel is the sensible ISO use case — a fresh dedicated box becoming the hub; note in comments that `agent` is the other valid value for someone repurposing this ISO to add a plain managed node instead) — OR, if late-commands turns out to be the wrong hook for something that needs to survive into the *running* system (worth checking Docker install ordering against the reference docs), use autoinstall's mechanism for dropping a systemd oneshot/first-boot unit instead. Use whichever the docs actually recommend for "run this once, after the base install, with network available" — don't guess, check.
- Reboot automatically when done.

## Task 2: ISO build script

Create `packaging/iso/build-iso.sh`. It should:
- Take the target Ubuntu Server release as a variable at the top (default to the current LTS, e.g. `24.04.x` — check what's actually current) and download the official `ubuntu-*-live-server-*.iso` from an official Ubuntu mirror URL if not already present in a local cache directory (these are ~2-3GB; don't re-download if already cached).
- Extract the ISO contents, copy in `autoinstall/user-data` and `autoinstall/meta-data`, and modify the boot config (GRUB for BIOS/UEFI, and isolinux/syslinux if the Ubuntu ISO still ships it for this release — check) to add the `autoinstall ds=nocloud;s=/cdrom/autoinstall/` kernel parameter to the default boot entry, per the quickstart doc's repacking instructions.
- Repack into a new bootable ISO via `xorriso` (match the exact xorriso invocation Ubuntu's own quickstart guide uses for hybrid BIOS+UEFI boot — don't hand-roll flags from memory, copy the documented approach) named `faroos-<version>-amd64.iso` in an output directory.
- `set -euo pipefail`, clear echo statements at each stage so someone running it manually understands what's happening and how long is left (image download and repack both take real time).

## Task 3: README for this directory

`packaging/iso/README.md`: explain what this ISO does and doesn't do (repeat the "not a custom distro, wipes the disk, Ubuntu Server underneath" framing from above so nobody's surprised), how to build it (`./build-iso.sh`), how to write it to a USB drive (`dd` or balenaEtcher, same as any Linux ISO), and how first boot proceeds (boots -> unattended install -> reboots -> Docker + FaroOS server running -> open `http://<ip>:8090` to finish setup).

## Task 4: CI workflow (manual trigger only)

`.github/workflows/build-iso.yml`: triggered via `workflow_dispatch` only (NOT on every push — this downloads gigabytes and takes a long time, shouldn't run automatically). Runs `packaging/iso/build-iso.sh` on `ubuntu-latest`, uploads the resulting ISO as a build artifact (GitHub Actions artifacts, not a release — attaching to releases can be a follow-up). Match the style/conventions of the existing `.github/workflows/ci.yml` (Go/action versions etc. aren't relevant here since there's no Go build, but keep the general YAML style consistent).

## Definition of done

- Fetch and actually read the three Ubuntu autoinstall doc URLs above before writing the YAML — this is exactly the kind of task where guessing the schema from memory produces subtly-wrong config that fails at install time on real hardware, which nobody in this conversation can test today.
- `bash -n packaging/iso/build-iso.sh` passes.
- `python3 -c "import yaml; yaml.safe_load(open('packaging/iso/autoinstall/user-data'))"` — note: autoinstall user-data files conventionally start with a `#cloud-config` header line before the YAML; account for that when checking (strip the header line, or use a YAML loader call that tolerates/ignores it — check how other real-world autoinstall user-data files handle this).
- You will NOT be able to actually run `build-iso.sh` end-to-end in this environment (multi-GB download, xorriso may not be installed, and there's no way to test-boot the resulting ISO here) — that's expected. Do whatever static/dry-run verification you can (script syntax, YAML validity, confirming the xorriso command matches Ubuntu's documented invocation, confirming referenced tool names like `xorriso`/`7z`/`isolinux` are the ones the actual quickstart guide uses), and report clearly what's verified vs. what can only be confirmed by actually running it on real hardware or in a VM later.

Do not run `git commit`. Leave changes staged/unstaged for review.
