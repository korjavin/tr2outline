# tr2outline

Lightweight middleware webhook receiver written in Go that catches webhooks from the **Anarlog** app, formats the meeting data into structured Markdown, and pushes it as a new document to your **Outline** Knowledge Base via its REST API.

---

## 🚀 Features

- **HMAC-SHA256 Signature Verification**: Validates `x-anarlog-signature` against the raw HTTP request body with constant-time comparison.
- **Event Filtering**: Only processes `note.enhanced` events (where AI summaries and action items are available); acknowledges other events (e.g., `webhook.test`) with `200 OK` without spamming Outline.
- **Structured Markdown Generation**:
  - Title: `Meeting: <Title> (<YYYY-MM-DD>)`
  - Meeting metadata: date and comma-separated participants.
  - Bullet-point summaries (`## 📝 Summary`).
  - Markdown checklists (`- [ ]`) for action items (`## ✅ Action Items`).
  - Meeting notes (`## 🗒️ Notes`).
  - Collapsible spoiler (`<details><summary>🎙️ Full Transcript</summary>...`) for the full audio transcript.
- **High Performance & Safe**: Written in idiomatic Go (Go 1.22+), multi-stage Docker image running under a non-root user (`appuser`).
- **Automated CI/CD**: Ready for GitHub Actions, GitHub Container Registry (`ghcr.io`), and Portainer webhook redeployment.

---

## ⚙️ Environment Variables

Create a `.env` file (see `.env.example`):

| Variable | Required | Default | Description |
|---|---|---|---|
| `PORT` | No | `3000` | HTTP port the server listens on |
| `ANARLOG_WEBHOOK_SECRET` | **Yes** | - | Webhook secret configured in Anarlog for HMAC-SHA256 signing |
| `OUTLINE_URL` | **Yes** | - | Base URL of your Outline instance (e.g. `https://app.getoutline.com` or self-hosted) |
| `OUTLINE_API_KEY` | **Yes** | - | Outline API Token (Settings -> API Tokens) |
| `OUTLINE_COLLECTION_ID` | **Yes** | - | UUID of the Outline collection where documents should be created |
| `DOMAIN` | No | - | Domain name if using Traefik reverse proxy |
| `TRAEFIK_NETWORK_NAME` | No | `traefik` | External Docker network name for Traefik |

---

## 💻 Local Development

### Prerequisites

- Go 1.22+
- (Optional) Docker

### Run with Go

1. Copy `.env.example` to `.env` and fill in your secrets:
   ```bash
   cp .env.example .env
   ```

2. Run tests:
   ```bash
   go test -v ./...
   ```

3. Run the service:
   ```bash
   go run .
   ```

The service will start listening on `http://localhost:3000`.

### Health Check

```bash
curl http://localhost:3000/health
# {"status":"ok"}
```

---

## 🧪 Testing the Webhook Locally

You can test the endpoint with `curl` using Python or OpenSSL to generate the HMAC-SHA256 signature.

### Sample test with bash & openssl:

```bash
SECRET="your_secret_here"
PAYLOAD='{
  "id": "evt_test",
  "event": "note.enhanced",
  "created_at": "2026-07-28T09:00:00.000Z",
  "data": {
    "meeting": {
      "id": "meet_123",
      "title": "Weekly Sync",
      "note": "Discussion on Q3 goals.",
      "summaries": ["Goal 1 approved", "Goal 2 under review"],
      "participants": ["Alice", "Bob"],
      "action_items": ["Alice to update documentation", "Bob to schedule client call"]
    },
    "transcript_text": "Alice: Hello everyone...\nBob: Hi Alice!"
  }
}'

# Compute HMAC-SHA256
SIG=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | awk '{print $2}')

# Send webhook
curl -X POST http://localhost:3000/api/webhooks/anarlog \
  -H "Content-Type: application/json" \
  -H "x-anarlog-signature: sha256=$SIG" \
  -H "x-anarlog-event: note.enhanced" \
  -d "$PAYLOAD"
```

---

## 🐳 Docker & Portainer Deployment

### Docker Run

```bash
docker build -t tr2outline:latest .
docker run -p 3000:3000 --env-file .env tr2outline:latest
```

### Docker Compose

```bash
docker compose up -d
```

### Portainer Stack & CI/CD Pipeline

This project implements the automated deployment workflow:

1. **GitHub Actions Workflow** (`.github/workflows/deploy.yml`):
   - Triggers on push to `master` (or `main`).
   - Builds and pushes a container image tagged with `${{ github.sha }}` to `ghcr.io/korjavin/tr2outline:<sha>`.
   - Switches to the `deploy` branch, updates `docker-compose.yml` with the exact SHA tag, commits and pushes to `deploy`.
   - Calls the Portainer Webhook to trigger auto-redeploy.

2. **Setup Steps in Portainer**:
   - In Portainer, create a new Stack: **Repository** method.
   - Repository URL: `https://github.com/korjavin/tr2outline`
   - Target branch: **`deploy`** (Portainer tracks the `deploy` branch, not `master`).
   - Compose path: `docker-compose.yml`
   - Enable **Webhook** and copy the webhook URL.
   - Configure Environment Variables in the Portainer Stack UI (`ANARLOG_WEBHOOK_SECRET`, `OUTLINE_URL`, `OUTLINE_API_KEY`, `OUTLINE_COLLECTION_ID`).

3. **Setup in GitHub**:
   - Go to your repository settings -> **Secrets and variables** -> **Actions**.
   - Add Secret: `PORTAINER_REDEPLOY_HOOK` with the Portainer webhook URL.
   - Ensure GitHub Actions workflow permissions have Read and Write access (**Settings -> Actions -> General -> Workflow permissions -> Read and write permissions**).
