# FaroOS install ISO

A bootable ISO for turning a blank machine into a FaroOS server with no manual install steps: boot it, wait, and you have FaroOS running.

## What this is — and isn't

This is **not** a custom Linux distribution or a custom kernel, the way ZimaOS is (built with Buildroot, a genuinely multi-month undertaking for a dedicated team — we looked at that path and deliberately ruled it out for this project). What this actually is: a stock **Ubuntu Server ISO, repacked with an [autoinstall](https://ubuntu.com/server/docs/install/autoinstall) config** (the same unattended-install mechanism Canonical documents and that tools like MAAS use) that answers every installer question non-interactively and, once the base OS is up, installs Docker and FaroOS via `packaging/install.sh` — the same script used for a normal curl-install.

Things to know before using it:

- **It wipes the target disk.** Whole-disk, no dual-boot, no "install alongside" — same as any OS installer image, including ZimaOS's own. Point it at a disk you don't need.
- It's Ubuntu Server underneath. You get a normal Ubuntu system (so apt, systemd, SSH — nothing hidden) with FaroOS running as a systemd service on top, not a sealed appliance image.
- Only `amd64` is built by this script right now.

## Building it

```sh
cd packaging/iso
./build-iso.sh
```

This downloads the official Ubuntu Server ISO (a few GB — cached in `.cache/` after the first run, so re-runs are fast), verifies its checksum against Ubuntu's published `SHA256SUMS`, injects the autoinstall config, and repacks a bootable ISO into `dist/`.

Useful environment variables:

| Variable | Default | What it does |
|---|---|---|
| `UBUNTU_VERSION` | `24.04.4` | Which Ubuntu Server point release to base this on |
| `FAROOS_COMPONENT` | `server` | `server` (the central panel — the normal choice for a dedicated box) or `agent` (make this box a plain managed node instead) |

> `agent` needs a pairing token generated from an existing panel (Dashboard → Add server), which obviously doesn't exist yet at ISO-build time. `install.sh` detects it's running non-interactively during autoinstall and, for `agent`, skips enabling the service and instead leaves instructions on the console for finishing setup by hand after first boot (see `/etc/faroos/agent.env` in `packaging/install.sh`). `server` needs no such secret and starts automatically — it's the better default for this ISO.
| `FAROOS_USERNAME` | `faroos-admin` | Local Linux account created on the installed system |
| `CACHE_DIR` / `OUTPUT_DIR` | `./.cache`, `./dist` | Where the downloaded ISO and build output go |

Requires `xorriso`, `isolinux` (for `/usr/lib/ISOLINUX/isohdpfx.bin`), `openssl`, and `curl` on the machine building the ISO — all standard on Debian/Ubuntu (`apt-get install xorriso isolinux openssl curl`).

The build also generates a random password for that local Linux account and writes it to `dist/faroos-<version>-amd64.CREDENTIALS.txt` next to the ISO — that file is the only copy, keep it. It's for console/SSH access to the underlying OS, unrelated to FaroOS's own admin login (which you set up separately in the browser).

## Writing it to a USB drive

Same as any Linux ISO:

```sh
sudo dd if=dist/faroos-24.04.4-amd64.iso of=/dev/sdX bs=4M status=progress conv=fsync
```

(or use balenaEtcher / Rufus if you'd rather not use `dd` — replace `/dev/sdX` with your actual USB device, and double check that device name, `dd` will happily overwrite the wrong disk.)

## What happens on first boot

1. Boot the target machine from the USB drive.
2. The installer runs unattended: partitions the disk, installs Ubuntu Server + Docker, no prompts.
3. Near the end of the install, it downloads and runs `packaging/install.sh <component>` against the freshly-installed system.
4. The machine reboots into the real system, where the FaroOS systemd service (enabled during install) starts automatically.
5. Open `http://<machine-ip>:8090` (for the `server` component) to finish FaroOS setup — creating the admin account — same first-run flow as any other install method.

## CI

`.github/workflows/build-iso.yml` builds this ISO in GitHub Actions, but only when triggered manually (`workflow_dispatch`) — it downloads several GB and takes a while, so it deliberately doesn't run on every push. The built ISO is uploaded as a workflow artifact.
