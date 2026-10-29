#!/usr/bin/env python3
"""Rehearse a tagged release in a disposable clone (D74).

    python3 scripts/rehearse-release.py

Clones the committed HEAD, tags the clone v0.0.0-rehearsal, builds every
archive with goreleaser with publishing skipped, and verifies them as the
release workflow does. The source checkout is never tagged, nothing is
pushed, and the clone is removed. Follows git-ticket-canvas's rehearsal. A
clone, not this checkout, because a linked git worktree records no VCS data
and its binaries would say dev.
"""
import subprocess
import tempfile
from pathlib import Path


def main():
    source = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip()
    with tempfile.TemporaryDirectory(prefix="vuoro-release-rehearsal-") as temp:
        clone = Path(temp) / "source"
        subprocess.run(["git", "clone", "--local", "--no-hardlinks", "--quiet", source, str(clone)], check=True)

        def run(*args):
            subprocess.run(args, cwd=clone, check=True, timeout=900)

        # goreleaser wants a remote it recognizes; nothing contacts it.
        run("git", "remote", "set-url", "origin", "https://github.com/terva-sh/vuoro.git")
        tag = "v0.0.0-rehearsal"
        run("git", "-c", "user.name=Release rehearsal", "-c", "user.email=rehearsal@example.invalid",
            "tag", "-a", tag, "-m", "A disposable local release rehearsal")
        run("goreleaser", "release", "--clean", "--skip=publish", "--parallelism=2")
        run("python3", "scripts/verify-release.py", "--tag", tag)
        print("The tagged rehearsal passed. The clone, its tag, and its archives are removed; nothing was published.")


if __name__ == "__main__":
    main()
