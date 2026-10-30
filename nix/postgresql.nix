# PostgreSQL for the host data system (docs/design/data-system-build-sheet-2026-10-30.md,
# decision 11). One cluster, two databases (control, graph), one application
# role (choir) whose password lives in /var/lib/go-choir/secrets/pg-app-password
# and reaches services through systemd LoadCredential (internal/dbcred.ApplyPGDSN).
#
# This module is self-contained and additive: it installs and configures
# PostgreSQL and the role, and nothing else depends on it. The Go services
# are wired to it (CHOIR_PG_CONTROL_DSN / CHOIR_PG_GRAPH_DSN, LoadCredential,
# After=) in nix/node-b.nix at the cutover, not here.
#
# Rollback: remove the import line in node-b.nix and deploy. The data
# directory (/var/lib/postgresql) is left in place.
{ config, lib, pkgs, ... }:
let
  pg = pkgs.postgresql_17;
  appUser = "choir";
  databases = [ "control" "graph" ];
  secretsDir = "/var/lib/go-choir/secrets";
  passwordFile = "${secretsDir}/pg-app-password";
in
{
  services.postgresql = {
    enable = true;
    package = pg;
    # Loopback TCP for the services (password, scram) plus the unix socket
    # for local maintenance as the postgres OS user.
    enableTCPIP = true;
    settings = {
      listen_addresses = lib.mkForce "127.0.0.1";
      port = 5432;
      unix_socket_directories = "/run/postgresql";
      max_connections = 200;
      shared_buffers = "1GB";
      wal_compression = "on";
      # Durability first: a committed event survives a power cut.
      synchronous_commit = "on";
      password_encryption = "scram-sha-256";
    };
    authentication = lib.mkForce ''
      # TYPE  DATABASE  USER      ADDRESS        METHOD
      local   all       postgres                 peer
      local   all       all                      peer
      host    all       ${appUser}  127.0.0.1/32 scram-sha-256
      host    all       ${appUser}  ::1/128      scram-sha-256
    '';
    ensureDatabases = databases;
    ensureUsers = [{ name = appUser; }];
  };

  # PostgreSQL may use at most 4 GiB of the host, page cache excluded.
  systemd.services.postgresql.serviceConfig.MemoryMax = "4G";

  # The application password and the role/database setup, idempotent on every
  # start. The password is generated once (hex, 0600, never printed) and set
  # through stdin, never a command line. Databases are created and owned here
  # as well, so the result does not depend on the order of the module's own
  # ensure* steps.
  systemd.services.go-choir-postgresql-roles = {
    description = "go-choir PostgreSQL application role, password and databases";
    wantedBy = [ "multi-user.target" ];
    after = [ "postgresql.service" "postgresql-setup.service" ];
    requires = [ "postgresql.service" ];
    path = [ pkgs.coreutils pkgs.openssl pkgs.util-linux pg ];
    serviceConfig = {
      Type = "oneshot";
      RemainAfterExit = true;
      User = "root";
    };
    script = ''
      set -euo pipefail
      install -d -m 0700 -o root -g root ${secretsDir}
      if [ ! -s ${passwordFile} ]; then
        ( umask 077
          openssl rand -hex 32 > ${passwordFile}.next
          mv -f ${passwordFile}.next ${passwordFile} )
        echo "go-choir PostgreSQL application password generated"
      fi
      chmod 0600 ${passwordFile}
      pw="$(cat ${passwordFile})"
      case "$pw" in ""|*[!A-Za-z0-9]*) echo "pg-app-password is empty or not alphanumeric" >&2; exit 1 ;; esac
      runuser -u postgres -- psql -X -q -v ON_ERROR_STOP=1 -h /run/postgresql -d postgres <<EOF >/dev/null
      SELECT 'CREATE ROLE ${appUser} LOGIN' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${appUser}') \gexec
      ALTER ROLE ${appUser} WITH LOGIN PASSWORD '$pw';
      ${lib.concatMapStringsSep "\n" (db: ''
      SELECT 'CREATE DATABASE ${db} OWNER ${appUser}' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${db}') \gexec
      ALTER DATABASE ${db} OWNER TO ${appUser};
      '') databases}
      EOF
      echo "go-choir PostgreSQL role ${appUser} and databases ${lib.concatStringsSep ", " databases} ensured"
    '';
  };
}
