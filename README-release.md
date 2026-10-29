# vuoro

vuoro is a desktop application and the front door to agentic code review. It
runs reviews with the agent tools you already have, and gives you one inbox
for every review you give and get across GitLab, Forgejo, and GitHub, ordered
by whose turn it is. The project is at <https://github.com/terva-sh/vuoro>.

This archive holds the `vuoro` binary, its license, the notices for the
software it includes (THIRD_PARTY_LICENSES), and this file.

## Install

Unpack the archive for your platform, put `vuoro` (`vuoro.exe` on Windows)
somewhere on your `PATH`, and run it. Check what you downloaded against the
release's `checksums.txt` first:

```sh
shasum -a 256 -c checksums.txt --ignore-missing
```

`vuoro -version` names the release and `vuoro -h` lists the options.

While its window is open, vuoro asks GitHub once a day whether there is a
newer release, and says so in the header. It downloads and installs one only
when you choose **Update**, and the new release runs once you quit and reopen
vuoro. `vuoro self-update` does the same from a terminal, and
`vuoro self-update -check` only asks. Either way the new archive is checked
against the release's `checksums.txt` before anything is replaced.

vuoro uses the web view your operating system provides:

- **macOS**: nothing to install.
- **Windows**: the Microsoft Edge WebView2 Runtime, which current Windows 10
  and 11 already have.
- **Linux**: WebKitGTK, with GTK 4 or GTK 3, from your distribution.

Agent reviews need an agent tool installed and signed in, such as Claude
Code. The inbox works without one.

## macOS: allowing an unsigned binary

This release of vuoro is not signed or notarized by Apple. When you open a
binary you downloaded with a browser, macOS refuses it with a message that
it cannot check the binary for malicious software. That is macOS saying it
does not know who made the binary, not that something is wrong with it.

If you choose to run it, allow it once in either of these ways:

- **System Settings.** Run `vuoro` once and dismiss the warning. Open System
  Settings, then Privacy & Security, scroll to Security, and choose
  **Open Anyway** beside the line naming vuoro. Run it again and confirm.
- **Terminal.** Remove the quarantine mark the browser added to the file:

  ```sh
  xattr -d com.apple.quarantine /path/to/vuoro
  ```

Only do either for a binary whose checksum matches `checksums.txt` from
<https://github.com/terva-sh/vuoro/releases>. You allow it once: a later
`vuoro self-update` downloads the new binary itself, so the new binary
carries no quarantine mark and macOS does not ask again.
