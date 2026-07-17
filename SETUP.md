# Environment Setup

Install these before Phase 1. Commands are grouped by OS — pick yours.

## macOS (Homebrew)

```bash
brew install --cask docker      # launch Docker Desktop once after installing
brew install kubectl kind helm go git jq node
python3 --version                # macOS ships Python 3; `brew install python@3.12` if it's old
```

## Linux (Ubuntu / Debian)

```bash
# Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER && newgrp docker

# kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl && sudo mv kubectl /usr/local/bin/

# kind
curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64
chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind

# helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Node via nvm (always gets latest LTS)
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash
nvm install --lts

# Go, Python, git
sudo apt update && sudo apt install -y golang-go python3 python3-pip python3-venv git
```

## Windows

Use WSL2:

```powershell
wsl --install
```

Then open your WSL Ubuntu shell and follow the Linux steps above. Install Docker Desktop on the Windows side with the WSL2 backend enabled (Settings → Resources → WSL Integration).

## Verify everything

```bash
docker version
kubectl version --client
kind version
helm version
node -v
python3 --version
go version
```

If any of these fail, fix it before starting Phase 1 — every later phase assumes all seven work.
