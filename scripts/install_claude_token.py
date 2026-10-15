#!/usr/bin/env python3
"""Install the owner's Claude setup-token on Node B without the clipboard dance.

  python3 scripts/install_claude_token.py            # prompts (hidden input)

Run it, then copy the token (it starts with sk-ant-oat01-) and paste it at the
hidden prompt. The token travels over ssh stdin only (never a command line,
never echoed, never printed) into /var/lib/go-choir/secrets/claude-oauth-token
(root, 0400). Whitespace from a wrapped terminal line is removed. The script
reports only the length and whether the prefix looks right.
docs/design/claude-delegate-slice1-2026-10-14.md.
"""
import getpass
import subprocess
import sys

DEST = "/var/lib/go-choir/secrets/claude-oauth-token"


def main():
    host = sys.argv[1] if len(sys.argv) > 1 else "node-b"
    token = "".join(getpass.getpass("Paste the Claude token (hidden): ").split())
    if not token.startswith("sk-ant-"):
        sys.exit("That does not start with sk-ant- ; nothing was installed. Copy only the token and try again.")
    if not (20 <= len(token) <= 4096) or any(c <= " " or c > "~" for c in token):
        sys.exit("That does not look like a token (length or characters); nothing was installed.")
    remote = f'sudo sh -c "umask 077; cat > {DEST}; chmod 0400 {DEST}; stat -c \'%a %U %s\' {DEST}"'
    r = subprocess.run(["ssh", host, remote], input=token.encode(), capture_output=True)
    if r.returncode != 0:
        sys.exit(f"ssh failed: {r.stderr.decode(errors='replace').strip()}")
    print(f"installed on {host}: {r.stdout.decode().strip()} (length {len(token)}, prefix ok)")


if __name__ == "__main__":
    main()
