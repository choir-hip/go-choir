# Lets Choir's Node B use this host as a Nix remote builder.
#
# Import this module from any NixOS config that manages the host (Choir's
# own or another project's). It grants exactly one thing: Node B's builder
# key may run `nix-daemon --stdio` as root, and nothing else (no shell, no
# forwarding). Node B points at it in nix/node-b.nix (nix.buildMachines,
# hostName "node-a-builder"); if this host is unreachable, Node B builds
# locally. See docs/node-a-shared-host.md.
{ ... }:
{
  users.users.root.openssh.authorizedKeys.keys = [
    ''restrict,command="nix-daemon --stdio" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEJ8EBaR/Mc4EtiLGKy8mDKy5H5Vey4VKpwpCHbSgKKT nix-builder@node-b''
  ];

  nix.settings = {
    experimental-features = [ "nix-command" "flakes" ];
    # Node B's builds declare these features (nix/node-b.nix buildMachines).
    system-features = [ "benchmark" "big-parallel" "kvm" "nixos-test" ];
  };
}
