#!/usr/bin/env python3
"""Write THIRD_PARTY_LICENSES: the notices for what a vuoro release ships.

    python3 scripts/third-party-licenses.py [--check]

A release archive carries the binary, and the binary carries other people's
code: the Go modules linked on any release target, the Go standard library,
Preact in the embedded frontend, and the design tokens the stylesheet is
generated from. Each one's license, and its NOTICE where it has one, is
copied from the module cache or node_modules, so the file is rebuilt rather
than edited. --check fails when the committed file differs, which is what
`just release-check` runs (D74).
"""
import json
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "THIRD_PARTY_LICENSES")
TARGETS = [("linux", "amd64"), ("linux", "arm64"), ("darwin", "amd64"),
           ("darwin", "arm64"), ("windows", "amd64")]
LICENSES = ("LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING")


def go(*args, env=None):
    return subprocess.run(["go", *args], cwd=ROOT, env=env, check=True,
                          capture_output=True, text=True).stdout


def linked_modules():
    """Every module linked into vuoro on any release target, but vuoro."""
    mods = {}
    for goos, goarch in TARGETS:
        env = dict(os.environ, GOOS=goos, GOARCH=goarch, CGO_ENABLED="0")
        out = go("list", "-deps", "-f",
                 "{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}", ".", env=env)
        for line in out.split("\n"):
            if line and not line.startswith("github.com/terva-sh/vuoro "):
                path, version = line.split(" ")
                mods[path] = version
    return mods


def module_dir(path, version):
    return json.loads(go("mod", "download", "-json", f"{path}@{version}"))["Dir"]


def design_version():
    out = go("list", "-m", "-f", "{{.Version}}", "github.com/terva-sh/design")
    return out.strip()


def notices(directory):
    """The license, and any NOTICE, in a directory, in that order."""
    texts = []
    for name in LICENSES:
        p = os.path.join(directory, name)
        if os.path.isfile(p):
            texts.append(open(p, encoding="utf-8").read().strip())
            break
    else:
        sys.exit(f"third-party-licenses: no license file in {directory}")
    p = os.path.join(directory, "NOTICE")
    if os.path.isfile(p):
        texts.append(open(p, encoding="utf-8").read().strip())
    return "\n\n".join(texts)


def section(title, body):
    return f"{title}\n{'=' * len(title)}\n\n{body}\n"


def build():
    parts = ["Third-party software distributed in vuoro\n\n"
             "The notices below cover the Go modules linked into vuoro on every release\n"
             "target, the Go standard library, the embedded frontend, and the design\n"
             "tokens its stylesheet is generated from. scripts/third-party-licenses.py\n"
             "writes this file.\n"]
    mods = linked_modules()
    # The Go project licenses the standard library and golang.org/x/sys under
    # the same text, and some Go installs (Homebrew's) ship no LICENSE in
    # GOROOT, so the text comes from x/sys on every builder. No Go version is
    # in the title: the builder's Go is not this file's to pin.
    sys_dir = module_dir("golang.org/x/sys", mods["golang.org/x/sys"])
    parts.append(section("The Go standard library", notices(sys_dir)))
    preact = os.path.join(ROOT, "node_modules", "preact")
    with open(os.path.join(preact, "package.json"), encoding="utf-8") as f:
        pv = json.load(f)["version"]
    parts.append(section(f"Preact {pv}", notices(preact)))
    dv = design_version()
    parts.append(section(f"github.com/terva-sh/design {dv}",
                         notices(module_dir("github.com/terva-sh/design", dv))))
    for path, version in sorted(mods.items()):
        parts.append(section(f"{path} {version}", notices(module_dir(path, version))))
    return "\n".join(parts)


def main(argv):
    text = build()
    if "--check" in argv:
        current = open(OUT, encoding="utf-8").read() if os.path.exists(OUT) else ""
        if current != text:
            sys.exit("THIRD_PARTY_LICENSES is out of date: run python3 scripts/third-party-licenses.py")
        return
    with open(OUT, "w", encoding="utf-8") as f:
        f.write(text)


if __name__ == "__main__":
    main(sys.argv[1:])
