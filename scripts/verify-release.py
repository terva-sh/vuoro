#!/usr/bin/env python3
"""Verify vuoro's release archives before anything is published (D74).

    python3 scripts/verify-release.py [--dist dist] [--tag vX.Y.Z]

Follows git-ticket-canvas's scripts/verify-release.py. It checks the exact set
of five archives and checksums.txt, the contents of each archive and that its
documents match the tree, and the build provenance go recorded in every
binary: the module, the commit at HEAD, the target, and with --tag the tag and
a clean tree. Then it runs the native binary, when this host is one of the
targets, and checks it reports the same release. Run it from the checkout
the archives were built from. Without --tag it checks a snapshot, whose
version is not a release.
"""
import argparse
import hashlib
import json
import platform
import re
import subprocess
import tarfile
import tempfile
import zipfile
from pathlib import Path

TARGETS = {"linux_amd64": "tar.gz", "linux_arm64": "tar.gz", "darwin_amd64": "tar.gz",
           "darwin_arm64": "tar.gz", "windows_amd64": "zip"}
DOCS = {"LICENSE", "THIRD_PARTY_LICENSES", "README-release.md"}
MODULE = "github.com/terva-sh/vuoro"


def check(condition, message):
    if not condition:
        raise SystemExit(f"verify-release: {message}")


def archive_files(path):
    if path.suffix == ".zip":
        with zipfile.ZipFile(path) as archive:
            names = archive.namelist()
            check(len(names) == len(set(names)), f"duplicate archive entries: {path.name}")
            return {name: archive.read(name) for name in names if not name.endswith("/")}
    with tarfile.open(path) as archive:
        members = archive.getmembers()
        names = [m.name for m in members]
        check(len(names) == len(set(names)), f"duplicate archive entries: {path.name}")
        check(all(m.isfile() or m.isdir() for m in members), f"non-file archive entry: {path.name}")
        return {m.name: archive.extractfile(m).read() for m in members if m.isfile()}


def checksums(path):
    result = {}
    for line in path.read_text().splitlines():
        match = re.fullmatch(r"([a-f0-9]{64})  ([A-Za-z0-9_.-]+)", line)
        check(match is not None, f"malformed checksum line: {line!r}")
        digest, name = match.groups()
        check(name not in result, f"duplicate checksum: {name}")
        result[name] = digest
    return result


def native_target():
    machine = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
    return f"{platform.system().lower()}_{machine}"


def verify(directory, tag):
    directory = Path(directory)
    if tag:
        check(re.fullmatch(r"v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?", tag), f"not a release tag: {tag}")
    version = json.loads((directory / "metadata.json").read_text())["version"]
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    if tag:
        check(version == tag[1:], f"the archives are version {version}, not {tag}")
        tagged = subprocess.check_output(["git", "rev-parse", f"{tag}^{{commit}}"], text=True).strip()
        check(tagged == commit, f"{tag} does not point at HEAD")
    expected = {f"vuoro_{version}_{target}.{ext}": target for target, ext in TARGETS.items()}
    sums = checksums(directory / "checksums.txt")
    check(set(sums) == set(expected), f"checksums.txt must name exactly the five archives, not {sorted(sums)}")
    actual = {p.name for p in directory.iterdir() if p.name.endswith((".tar.gz", ".zip"))}
    check(actual == set(expected), f"missing or extra archives: {sorted(actual ^ set(expected))}")
    native = native_target()
    with tempfile.TemporaryDirectory(prefix="vuoro-release-verify-") as temp:
        root = Path(temp)
        ran = None
        for name, target in sorted(expected.items()):
            path = directory / name
            check(hashlib.sha256(path.read_bytes()).hexdigest() == sums[name], f"checksum mismatch: {name}")
            files = archive_files(path)
            binary_name = "vuoro.exe" if target.startswith("windows") else "vuoro"
            check(set(files) == DOCS | {binary_name}, f"unexpected contents in {name}: {sorted(files)}")
            for doc in DOCS:
                check(files[doc] == Path(doc).read_bytes(), f"{doc} in {name} differs from the tree")
            unpacked = root / target
            unpacked.mkdir()
            binary = unpacked / binary_name
            binary.write_bytes(files[binary_name])
            binary.chmod(0o755)
            info = subprocess.check_output(["go", "version", "-m", str(binary)], text=True)
            check(f"\tpath\t{MODULE}\n" in info, f"{name} is not built from {MODULE}")
            check(f"vcs.revision={commit}" in info, f"{name} is not built from HEAD")
            goos, goarch = target.split("_")
            check(f"GOOS={goos}" in info and f"GOARCH={goarch}" in info, f"{name} is built for another target")
            check("CGO_ENABLED=0" in info, f"{name} is built with cgo")
            if tag:
                check(f"\tmod\t{MODULE}\t{tag}\t" in info, f"{name} does not record {tag}")
                check("vcs.modified=false" in info, f"{name} is built from a modified tree")
            if target == native:
                got = subprocess.check_output([str(binary), "-version"], text=True, timeout=30).strip()
                if tag:
                    check(got == f"vuoro {tag}", f"the native binary says {got!r}, not vuoro {tag}")
                review = subprocess.run([str(binary), "review", "-h"], capture_output=True, text=True, timeout=30)
                check(review.returncode == 0 and "vuoro review" in review.stderr,
                      f"the native binary's review command did not answer: {review.stderr}")
                ran = got
    print(f"Verified five archives, checksums, documents and provenance for {version}"
          + (f"; the native binary reports {ran!r}." if ran else "; no archive is native to this host."))


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--dist", default="dist")
    parser.add_argument("--tag", help="the release tag; omit only for a snapshot")
    args = parser.parse_args()
    verify(args.dist, args.tag)
