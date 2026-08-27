#!/usr/bin/env python3
"""Check a built Desktop extension.

Claude Desktop reads manifest.json, picks the command for the platform, and
runs it. This script does the same, so a missing or broken binary fails here
rather than on a user's machine.

Usage:
    python3 scripts/test-extension.py [path/to/tekmetric-mcp.mcpb]
"""

import json
import os
import platform
import shutil
import stat
import subprocess
import sys
import tempfile
import zipfile

DEFAULT_ARCHIVE = os.path.join("dist", "tekmetric-mcp.mcpb")

# A launcher chooses a build by architecture, which the manifest cannot express.
# The shell launcher names its builds in readable text. The Windows launcher is
# a compiled program, so its builds are listed here.
LAUNCHER_TARGETS = {
    "tekmetric-mcp-windows.exe": [
        "tekmetric-mcp-windows-amd64.exe",
        "tekmetric-mcp-windows-arm64.exe",
    ],
}

# Claude Desktop keys platform_overrides the way Node names platforms.
PLATFORM_KEYS = {"Darwin": "darwin", "Linux": "linux", "Windows": "win32"}

failures = []


def fail(message):
    failures.append(message)
    print(f"  FAIL  {message}")


def ok(message):
    print(f"  ok    {message}")


def command_for(mcp_config, platform_key):
    """Return the command Claude Desktop runs on a platform."""
    override = mcp_config.get("platform_overrides", {}).get(platform_key)
    if override and "command" in override:
        return override["command"]
    return mcp_config["command"]


def binary_name(command):
    return command.replace("${__dirname}/", "")


def launcher_targets(path, names):
    """Return the archive members a launcher runs.

    A compiled launcher is listed in LAUNCHER_TARGETS. A shell launcher starts
    with a shebang, and any archive member it names is a build it may hand
    control to. Each one has to be present and executable.
    """
    declared = LAUNCHER_TARGETS.get(os.path.basename(path))
    if declared is not None:
        return declared

    try:
        with open(path, "rb") as handle:
            if handle.read(2) != b"#!":
                return []
            handle.seek(0)
            text = handle.read().decode("utf-8", "replace")
    except OSError:
        return []

    return sorted(
        name
        for name in names
        if name != os.path.basename(path) and "/" + name in text
    )


def main():
    archive_path = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_ARCHIVE

    if not os.path.exists(archive_path):
        print(f"ERROR: {archive_path} does not exist. Run 'make extension' first.")
        return 1

    print(f"Checking {archive_path}")
    size_mb = os.path.getsize(archive_path) / (1024 * 1024)
    print(f"  size  {size_mb:.1f}MB\n")

    workdir = tempfile.mkdtemp()
    try:
        return check(archive_path, workdir)
    finally:
        shutil.rmtree(workdir, ignore_errors=True)


def check(archive_path, workdir):
    with zipfile.ZipFile(archive_path) as archive:
        names = set(archive.namelist())

        if "manifest.json" not in names:
            print("  FAIL  the archive holds no manifest.json")
            return 1

        manifest = json.loads(archive.read("manifest.json"))
        archive.extractall(workdir)

        # extractall drops the mode bits, so read them from the archive and put
        # them back. The launcher checks that the build it runs is executable.
        modes = {i.filename: (i.external_attr >> 16) for i in archive.infolist()}
        for member, mode in modes.items():
            target = os.path.join(workdir, member)
            if mode and os.path.isfile(target):
                os.chmod(target, mode & 0o777)

    mcp_config = manifest["server"]["mcp_config"]

    print("Commands the manifest names:")
    for label, key in [("macOS", "darwin"), ("Linux", "linux"), ("Windows", "win32")]:
        name = binary_name(command_for(mcp_config, key))

        if name not in names:
            fail(f"{label} runs {name}, which the archive does not hold")
            continue
        if not modes.get(name, 0) & stat.S_IXUSR:
            fail(f"{label} runs {name}, which is not executable")
            continue
        ok(f"{label} runs {name}")

        # A command may be a launcher that picks a build by architecture. The
        # manifest cannot name those builds, so read them out of the script.
        for target in launcher_targets(os.path.join(workdir, name), names):
            if target not in names:
                fail(f"{name} runs {target}, which the archive does not hold")
            elif not modes.get(target, 0) & stat.S_IXUSR:
                fail(f"{name} runs {target}, which is not executable")
            else:
                ok(f"  {name} can run {target}")

    print("\nSettings the extension passes:")
    env = mcp_config.get("env", {})
    for name in ("TEKMETRIC_CLIENT_ID", "TEKMETRIC_CLIENT_SECRET",
                 "TEKMETRIC_BASE_URL", "TEKMETRIC_DEFAULT_SHOP_ID"):
        if name in env:
            ok(name)
        else:
            fail(f"the extension does not pass {name}")

    base_url = manifest.get("user_config", {}).get("base_url", {}).get("default", "")
    if base_url == "https://shop.tekmetric.com":
        ok(f"base_url default is {base_url}")
    else:
        fail(f"base_url default is {base_url}, want https://shop.tekmetric.com")

    print("\nRunning the binary for this machine:")
    platform_key = PLATFORM_KEYS.get(platform.system())
    if platform_key is None:
        print(f"  skip  {platform.system()} is not a Desktop platform")
    else:
        name = binary_name(command_for(mcp_config, platform_key))
        binary = os.path.join(workdir, name)

        if not os.path.exists(binary):
            fail(f"{name} is missing, so it cannot run here")
        else:
            try:
                result = subprocess.run(
                    [binary, "version"], capture_output=True, text=True, timeout=30
                )
            except OSError as err:
                fail(f"{name} did not start: {err}")
            else:
                if result.returncode != 0:
                    fail(f"{name} exited {result.returncode}: {result.stderr.strip()}")
                else:
                    output = (result.stdout + result.stderr).strip().splitlines()
                    ok(f"{name} runs: {output[0] if output else 'no output'}")

    print()
    if failures:
        print(f"{len(failures)} check(s) failed. The extension is not ready to publish.")
        return 1

    print("Every check passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
