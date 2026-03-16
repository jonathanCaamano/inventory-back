# Repository Setup

Configuration required to make CI/CD work end-to-end.
Set everything under **Settings → Secrets and variables → Actions**.

---

## GitHub Secrets

> Secrets are encrypted. Never put real values in code or commit messages.

| Secret | Description | How to generate |
|---|---|---|
| `KUBECONFIG` | kubeconfig of the target cluster, **base64-encoded** | `base64 -w0 ~/.kube/config` |
| `DB_PASSWORD` | PostgreSQL password for the `inventory` user | Choose a strong password |
| `JWT_SECRET` | JWT signing key — **minimum 32 characters** | `openssl rand -hex 32` |
| `MINIO_ACCESS_KEY` | MinIO access key | Set in MinIO admin panel |
| `MINIO_SECRET_KEY` | MinIO secret key | Set in MinIO admin panel |
| `GHCR_PAT` | Personal Access Token for the cluster to pull images from GHCR | [Create PAT](https://github.com/settings/tokens) with scope `read:packages` |

---

## GitHub Variables

> Variables are not encrypted. Safe for non-sensitive config.

| Variable | Description | Example |
|---|---|---|
| *(none required for this repo)* | — | — |

---

## Environments

The CD workflow uses a GitHub Environment named **`production`**.

Go to **Settings → Environments → production** and optionally configure:
- **Required reviewers** — people who must approve each deployment
- **Wait timer** — delay before the deployment job starts
- **Deployment branches** — restrict to `main` or tag patterns (`v*`)

---

## Image registry

Images are pushed to **GitHub Container Registry (GHCR)**:

```
ghcr.io/jonathancaamano/inventory-back:<tag>
```

Authentication uses the automatic `GITHUB_TOKEN` (no extra secret needed to push).
The `GHCR_PAT` secret is only needed by the Kubernetes cluster to pull the image.

### Tag strategy

| Trigger | Tags applied |
|---|---|
| Push to `main` | `latest`, `sha-<short-sha>` |
| Push tag `v1.2.3` | `1.2.3`, `1.2`, `latest` |

---

## Kubernetes Ingress

The API is exposed via the cluster's nginx ingress controller:

| Domain | Service |
|---|---|
| `invent-back.jcrlabs.net` | `inventory-back-service:8080` |

Make sure a DNS `A` record points `invent-back.jcrlabs.net` to the cluster's ingress IP.

---

## Kubernetes secrets applied by CD

The deploy job creates/updates these K8s resources automatically from the GitHub Secrets above:

```bash
# Application secret  (namespace: taller-inventario)
# MinIO credentials are hardcoded (minioadmin) — no GitHub secret needed.
kubectl create secret generic inventory-back-secrets \
  --from-literal=DB_USER=inventory \
  --from-literal=DB_PASSWORD="<DB_PASSWORD>" \
  --from-literal=JWT_SECRET="<JWT_SECRET>" \
  --from-literal=MINIO_ACCESS_KEY="minioadmin" \
  --from-literal=MINIO_SECRET_KEY="minioadmin" \
  --namespace=taller-inventario

# Image pull secret  (namespace: taller-inventario)
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=<actor> \
  --docker-password="<GHCR_PAT>"
```

Both commands are run with `--dry-run=client -o yaml | kubectl apply -f -` so they are **idempotent** — safe to run on every deployment.

---

## Local development

Copy `.env.example` to `.env` and fill in the values:

```bash
cp .env.example .env
```

Start dependencies with Docker Compose (if available) then:

```bash
make run
```
