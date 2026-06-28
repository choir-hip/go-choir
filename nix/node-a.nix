# Disposable go-choir Node A design lab for https://choir-ip.com.
{ lib, ... }:
{
  imports = [ ./node-b.nix ];

  networking.hostName = lib.mkForce "go-choir-a";

  services.caddy.virtualHosts = lib.mkForce {
    "choir-ip.com" = {
      extraConfig = ''
        handle /auth/* {
          reverse_proxy 127.0.0.1:8081
        }
        handle /health {
          reverse_proxy 127.0.0.1:8082
        }
        handle /api/email/resend/webhook {
          reverse_proxy 127.0.0.1:8087
        }
        handle /api/* {
          reverse_proxy 127.0.0.1:8082 {
            transport http {
              response_header_timeout 15m
              read_timeout 15m
              write_timeout 15m
            }
          }
        }
        handle /provider/* {
          respond "provider routes are not available from the public edge" 403
        }
        handle /internal/* {
          respond "internal routes are not available from the public edge" 403
        }
        handle /assets/* {
          root * /var/www/go-choir/frontend-current
          header Cache-Control "public, max-age=31536000, immutable"
          file_server
        }
        handle {
          root * /var/www/go-choir/frontend-current
          header Cache-Control "no-store"
          try_files {path} /index.html
          file_server
        }
      '';
    };
  };

  systemd.services.go-choir-auth.serviceConfig.Environment = lib.mkForce [
    "AUTH_PORT=8081"
    "AUTH_DB_PATH=/var/lib/go-choir/auth/auth.db"
    "AUTH_RP_ID=choir-ip.com"
    "AUTH_RP_ORIGINS=https://choir-ip.com"
    "AUTH_JWT_PRIVATE_KEY_PATH=/var/lib/go-choir/auth-signing/ed25519-key"
    "AUTH_ACCESS_TOKEN_TTL=5m"
    "AUTH_REFRESH_TOKEN_TTL=720h"
    "AUTH_COOKIE_SECURE=true"
  ];

  systemd.services.go-choir-maild.serviceConfig.Environment = lib.mkForce [
    "SERVER_HOST=0.0.0.0"
    "MAILD_PORT=8087"
    "MAILD_DB_PATH=/var/lib/go-choir/mail/mail.db"
    "MAILD_STORAGE_ROOT=/var/lib/go-choir/mail"
    "MAILD_PRIMARY_DOMAIN=choir-ip.com"
    "MAILD_RUNTIME_URL=http://127.0.0.1:8085"
    "MAILD_VMCTL_URL=http://127.0.0.1:8083"
  ];
}
