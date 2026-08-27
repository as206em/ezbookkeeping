# Production deployment

The `Deploy` GitHub Actions workflow builds the application image, publishes
both `sha-<commit>` and `latest` tags to
`ghcr.io/as206em/ezbookkeeping`, pins `portainer.yaml` to the immutable SHA tag,
and calls Portainer to update the stack.

Before the first deployment:

1. Create external Docker secrets named `ezbookkeeping_secret_key` and
   `ezbookkeeping_postgres_password`. Generate the signing-key value with
   `ezbookkeeping security gen-secret-key` and keep it stable; changing it
   invalidates existing signed sessions.
2. Ensure the existing `azure_storage_account` and `azure_storage_key` Docker
   secrets are available to the stack. The maintenance container backs up the
   PostgreSQL database to the `backups/ezbookkeeping` Azure Blob prefix at
   02:00 and 14:00 each day.
3. Ensure the external `traefik_public` overlay network exists and Traefik uses
   an entrypoint named `websecure` and a certificate resolver named
   `letsencrypt`.
4. Create the Portainer stack from `deploy/portainer.yaml` and configure its Git
   repository webhook as the GitHub Actions secret `PORTAINER_WEBHOOK`.
5. If either GHCR package is private, configure Portainer with registry credentials
   that can pull `ghcr.io/as206em/ezbookkeeping`.
6. Point the DNS record for `fi.asem-mkl.com` at the Traefik host.

PostgreSQL data and uploaded files are kept in named volumes. Application logs
are written only to stdout/stderr for collection by Docker, Portainer, or the
observability stack. On a multi-node Swarm, constrain the stateful services to
the node that owns their volumes or replace local volumes with shared storage.
