# Rollwave

**The missing deployment tool for Docker Swarm.**

[![Go Report Card](https://goreportcard.com/badge/github.com/rollwave-dev/rollwave)](https://goreportcard.com/report/github.com/rollwave-dev/rollwave)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
![Rollwave Demo](demo.gif)

Rollwave brings the developer experience of modern PaaS (like Vercel or Heroku) to your own servers running Docker Swarm. It handles the complexity of zero-downtime deployments, secret rotation, and build pipelines, so you don't have to write messy Bash scripts.

> ⚠️ **Status: Alpha / MVP.** APIs and behavior may change.

## Why Rollwave?

Docker Swarm is excellent for simple orchestration, but it lacks modern tooling. Rollwave solves the biggest pain points:

- **🔐 Zero-Downtime Secret Rotation:** Native Swarm services cannot easily rotate secrets without downtime. Rollwave implements an *Immutable Secret Pattern*, hashing your secrets and updating services seamlessly.
- **🏗️ Integrated Build Pipeline:** No need for separate CI scripts. Rollwave reads your `docker-compose.yml`, builds your images, pushes them to your registry, and deploys them in one go.
- **🧹 Auto-Cleanup:** Automatically prunes old, unused secrets to keep your cluster clean.
- **📄 Single Source of Truth:** Uses your existing `docker-compose.yml` as the definition for both building and deploying.

## Installation

### Option 1: Download Binary (Recommended)

You can download the pre-compiled binary for your operating system (Linux, macOS, Windows) from the **[Releases page](https://github.com/rollwave-dev/rollwave/releases)**.

**Linux / macOS:**
1. Download the archive (e.g., `rollwave_..._linux_amd64.tar.gz`).
2. Extract the binary.
3. Move it to your path:
   ```bash
   tar -xvf rollwave_*.tar.gz
   sudo mv rollwave /usr/local/bin/
   ```

**Windows:**
1. Download the `.zip` archive.
2. Extract it and add the folder to your PATH.

### Option 2: Build from Source

If you have Go 1.22+ installed, you can build the latest version directly:

```bash
git clone https://github.com/rollwave-dev/rollwave.git
cd rollwave
go build -o rollwave ./cmd/rollwave

# Optional: Move to your path
sudo mv rollwave /usr/local/bin/
```

## Quick Start

### 1. Initialize a Project

Go to your project directory (where your `docker-compose.yml` is) and run:

```bash
rollwave init
```

This creates a `rollwave.yml` configuration file. Edit it to match your project name.

### 2. Configure Secrets

Rollwave reads secrets from your environment or a `.env` file. Any variable starting with `ROLLWAVE_SECRET_` will be processed.

**`.env`**
```bash
# Define your secrets here
ROLLWAVE_SECRET_DB_PASSWORD="super-secret-password"
ROLLWAVE_SECRET_API_KEY="abcdef123456"
```

**`docker-compose.yml`**
Reference these secrets in your compose file using their logical names (without the prefix):

```yaml
version: "3.8"
services:
  web:
    image: my-registry.com/my-app
    build: 
      context: .
    secrets:
      - source: DB_PASSWORD
        target: db_password

secrets:
  DB_PASSWORD:
    external: true
```

### 3. Deploy

To build your image, push it, sync secrets, and deploy to Swarm:

```bash
# Ensure you are pointing to your Swarm manager
export DOCKER_HOST=ssh://user@your-swarm-manager

# Run the magic
rollwave deploy --build
```

**What happens behind the scenes:**
1.  **Build:** Rollwave builds the image defined in `docker-compose.yml` and tags it with the git short hash.
2.  **Push:** Pushes the image to your registry.
3.  **Secrets:** Hashes the values in `.env`. If `DB_PASSWORD` changed, it creates a new Swarm secret `my-stack_prod_DB_PASSWORD_<hash>`.
4.  **Rewrite:** Generates a temporary Compose file mapping the service to the new specific secret version and image tag.
5.  **Deploy:** Runs `docker stack deploy`. Swarm detects the configuration change and performs a rolling update.

## Private Registries

If your images are stored in a private registry (GitHub Container Registry, GitLab Registry, AWS ECR, etc.), you need to provide credentials so Rollwave can push the image and Swarm can pull it.

Set the following environment variables (e.g., in your CI/CD pipeline):

```bash
export ROLLWAVE_REGISTRY_USER="your-username"
export ROLLWAVE_REGISTRY_PASSWORD="your-token-or-password"
```

When you run `rollwave deploy --build`, the tool will:
1. Automatically log in to the registry defined in your image name.
2. Push the built image.
3. Pass the authentication credentials to the Swarm cluster so it can pull the image securely.

### 4. Cleanup

Over time, secret rotation creates many versions. Clean them up safely:

```bash
rollwave prune
```

This checks which secrets are currently used by running services and deletes the rest.

## Configuration (`rollwave.yml`)

```yaml
version: v1
project: my-project

stack:
  name: my-stack
  compose_file: docker-compose.yml

secrets:
  stack_prefix: prod # Prefixes secrets like: stack_prod_KEY_hash

deploy:
  with_secrets: true # Automatically sync secrets on deploy
```

## Roadmap

- [x] Support for Private Registry Authentication (`docker login` / config.json)
- [ ] Automatic `prune` after successful deploy
- [ ] Multi-environment support (staging/production in one config)
- [ ] Binary releases via Homebrew

## License

MIT