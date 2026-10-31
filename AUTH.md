# Pursuit auth for agents

The guide lives in [internal/spec/auth.md](internal/spec/auth.md) so the server can embed it. A running server serves it at `GET /auth.md`.

Short version: send `Authorization: Bearer $PURSUIT_TOKEN` with a `read` or `read,write` token a human made for you. Never hold `approve`.
