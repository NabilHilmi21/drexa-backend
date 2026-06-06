# Drexa Backend — Claude Context

> **This repository contains only the Go backend service.**
> The Next.js 14 frontend is a separate repository/service and is not present here.

## Full System Architecture (for context)

The diagram below shows the complete Drexa Trading platform. This repo covers
everything from **GATEWAY** down (Go API, data stores, infra, deploy).

```
┌─────────────────────────────────────────────────────────────────────┐
│ CLIENT  [separate repo]                                             │
│  Browser/web (Next.js 14 — App Router)   Mobile (future, React     │
│                                          Native / PWA)             │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────────┐
│ NEXT.JS PAGES  [separate repo]                                      │
│  Auth (login · register · OTP)     KYC mocked (form · ID upload)   │
│  Dashboard (portfolio · PnL)       Market (/market/[symbol])       │
│  Trade (order form · book)         Wallet (deposit · withdraw)     │
│  Orders (history · open)           Admin (users · KYC review)      │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
╔════════════════════════════════▼════════════════════════════════════╗
║ GATEWAY  ◄─── THIS REPO STARTS HERE                                ║
║  Go API gateway                        Firebase Admin               ║
║  (JWT verify · rate limit ·            (ID token verify)           ║
║   routing · CORS)                                                   ║
╚════════════════════════════════╦════════════════════════════════════╝
                                 ║
┌────────────────────────────────▼────────────────────────────────────┐
│ GO BACKEND — CLEAN ARCHITECTURE                                     │
│  Auth (JWT · bcrypt · OTP)         KYC (mocked · status flag)      │
│  User (profile · security)         Market data (prices · WebSocket)│
│  Order (limit · market · book)     Wallet (balance · tx history)   │
│  Payment (Stripe · fiat on/off)    Notification (Twilio · SendGrid)│
└────────────────────────────────┬────────────────────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────────┐
│ DATA                                                                │
│  MySQL                    Redis                    Firebase         │
│  (users · orders ·        (OTP · sessions ·        (identity ·     │
│   wallets · kyc)           rate limit)              auth state)    │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────────┐
│ INFRA — GOOGLE CLOUD PLATFORM (GKE Cluster)                        │
│  Go pods (HPA · rolling deploy)     MySQL pod (PersistentVolume)   │
│  Redis pod (in-memory · sidecar)    Nginx ingress (TLS · LB)       │
│  K8s Secrets (DB creds · API keys)  Cloud Storage (KYC · avatars)  │
│  Cloud CDN (static assets)          Cloud Monitoring (logs/alerts) │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────────┐
│ DEPLOY — GitHub Actions · Docker · Kubernetes                      │
│  GitHub (monorepo · PRs)                                           │
│  → Actions CI (test · lint · build)                                │
│  → Docker build (image → GCR)                                      │
│  → kubectl apply (rolling update)                                  │
│  Environments: dev (local · hot reload) | staging (auto on PR) |   │
│                production (main branch · GKE)                      │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────────┐
│ EXTERNAL SERVICES                                                   │
│  Stripe (deposit · withdrawal)     Twilio (SMS OTP · alerts)       │
│  SendGrid (email verify · notify)  Market data API (CoinGecko /    │
│                                    Binance)                        │
└─────────────────────────────────────────────────────────────────────┘
```

## Key Technical Decisions

- **Language**: Go, clean architecture (domain → usecase → repository → delivery)
- **Auth**: Firebase Admin SDK for ID token verification; JWT + bcrypt + OTP for API auth
- **Database**: MySQL via GORM (primary data store)
- **Cache**: Redis (OTP storage, sessions, rate limiting)
- **Deployment**: GKE on Google Cloud Platform, Docker, GitHub Actions CI/CD
- **Market data**: CoinGecko / Binance API integration via WebSocket
- **Payments**: Stripe for fiat deposit/withdrawal
- **Notifications**: Twilio (SMS), SendGrid (email) — currently mocked
- **KYC**: Currently mocked with a status flag

## Local Development (Docker)

```bash
cp .example-env .env   # fill in credentials
docker compose up      # starts MySQL + Redis + Go server
```

Server listens on `:8080`. MySQL on `3306`, Redis on `6379`.

## Firebase Setup

1. Firebase Console → Project Settings → Service Accounts → **Generate New Private Key**
2. Base64-encode the downloaded JSON:
   - Linux/Mac: `base64 -w 0 service-account.json`
   - Windows: `[Convert]::ToBase64String([IO.File]::ReadAllBytes("service-account.json"))`
3. Paste the result into `FIREBASE_CREDENTIALS_JSON` in `.env`
