# Buoy

Buoy is a Kubernetes controller and web dashboard designed to manage, schedule, and trigger rolling updates for your cluster workloads. It provides a centralized command center to safely restart Deployments, StatefulSets, and DaemonSets.

## Features

- **Cluster Watching**: Monitors your Kubernetes cluster for changes to `Deployments`, `StatefulSets`, and `DaemonSets`.
- **Web Dashboard**: Built-in web interface (port `8080`) to view and manage observed resources.
- **Manual Approvals**: Approve and trigger rolling updates directly through the dashboard or REST API.
- **Graceful Rollouts**: Triggers standard Kubernetes rolling updates by patching the workload with a `restartedAt` annotation (identical to `kubectl rollout restart`).
- **Automated Scheduling**: Built-in scheduler for managing automated or delayed rollouts.

## How It Works

1. **Informers**: Connects to the Kubernetes API and sets up informers to watch for workload events.
2. **Registry**: Maintains an internal state registry of all observed resources.
3. **HTTP Server**: Serves an embedded frontend dashboard and exposes an API (`/api/resources/approve`) to handle user approvals.
4. **Updater**: Dispatches Kubernetes strategic merge patches to the respective workload controller, instructing it to gracefully rotate pods.

## Quick Start

### Prerequisites
- Go 1.21+
- Access to a Kubernetes cluster (`~/.kube/config` or in-cluster SA)

### Running Locally
```bash
go run .
```
The web dashboard will be available at `http://localhost:8080`.

## API Endpoints

- `GET /`: The main web dashboard.
- `GET /api/resources`: Returns a JSON list of all observed resources.
- `POST /api/resources/approve`: Approves and triggers a rolling update. Expects a JSON payload with `namespace`, `kind`, and `name`.
- `GET /health`: Health check endpoint.
