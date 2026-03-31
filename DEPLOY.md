# Buoy Deployment Guide

## Prerequisites

- Kubernetes cluster (1.20+)
- kubectl configured
- Traefik ingress controller with forward-auth middleware configured
- Docker registry access (for building and pushing images)

## Quick Start

### 1. Build and Push Docker Image

```bash
# Build the image
docker build -t your-registry/buoy:latest .

# Push to your registry
docker push your-registry/buoy:latest
```

### 2. Update Deployment Configuration

Edit `deploy/deployment.yaml` and update the image reference:

```yaml
image: your-registry/buoy:latest
```

Edit `deploy/ingress.yaml` and update the host:

```yaml
- host: buoy.your-domain.com
```

### 3. Deploy to Kubernetes

```bash
# Create namespace and RBAC
kubectl apply -f deploy/rbac.yaml

# Deploy Buoy
kubectl apply -f deploy/deployment.yaml

# Create service
kubectl apply -f deploy/service.yaml

# Create ingress (optional, for web UI access)
kubectl apply -f deploy/ingress.yaml
```

### 4. Verify Deployment

```bash
# Check pod status
kubectl get pods -n buoy-system

# Check logs
kubectl logs -n buoy-system -l app=buoy -f

# Port-forward for local testing (optional)
kubectl port-forward -n buoy-system svc/buoy 8080:80
```

Access the dashboard at: `http://localhost:8080`

## Annotating Resources

### Enable Monitoring

Add annotations to your Deployments, StatefulSets, or DaemonSets:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  annotations:
    # Required: Enable Buoy monitoring
    buoy.sh/watch: "true"
    
    # Optional: Check schedule (default: @hourly)
    buoy.sh/watchSchedule: "@hourly"  # or "@daily"
    
    # Optional: Update approval (default: auto)
    buoy.sh/updateApproval: "required"  # or "auto"
```

### Annotation Reference

| Annotation | Values | Default | Description |
|------------|--------|---------|-------------|
| `buoy.sh/watch` | `"true"` | - | **Required** - Enable Buoy monitoring |
| `buoy.sh/watchSchedule` | `"@hourly"`, `"@daily"` | `"@hourly"` | How often to check for updates |
| `buoy.sh/updateApproval` | `"required"`, `"auto"` | `"auto"` | Whether updates need manual approval |

## Approval Workflow

### Auto-Update Mode (`buoy.sh/updateApproval: "auto"`)

When a new image SHA is detected:
1. Buoy logs the mismatch
2. **Automatically triggers rolling update**
3. Kubernetes pulls the new image
4. Pods restart with the new image

### Manual Approval Mode (`buoy.sh/updateApproval: "required"`)

When a new image SHA is detected:
1. Buoy logs the mismatch
2. **Marks resource as "Pending Approval" in web UI**
3. Admin reviews the update in the dashboard
4. Admin clicks "🚀 Approve & Update" button
5. Buoy triggers rolling update
6. Kubernetes pulls the new image
7. Pods restart with the new image

## Web UI Access

### With Traefik Forward-Auth

The ingress is pre-configured for Traefik forward-auth:

```yaml
annotations:
  traefik.ingress.kubernetes.io/router.middlewares: auth-system-forward-auth@kubernetescrd
```

Update the middleware reference to match your Keycloak/auth setup.

### Without Authentication (Development Only)

For local development, you can remove the forward-auth annotation or use port-forwarding:

```bash
kubectl port-forward -n buoy-system svc/buoy 8080:80
```

## Monitoring

### Health Checks

Buoy exposes a health endpoint:

```bash
curl http://buoy.buoy-system.svc.cluster.local/health
```

Response:
```json
{"status":"healthy"}
```

### Logs

Buoy uses structured logging with emojis for easy parsing:

```bash
kubectl logs -n buoy-system -l app=buoy -f
```

Example log output:
```
🚢 Buoy is leaving the dock: Starting informers...
⚓ Informers started and synced
⚓ Watching resource name=my-app namespace=default kind=Deployment schedule=@hourly requires_approval=true
🔍 Running initial SHA comparison check...
🔄 Image SHA mismatch detected resource=my-app container=web live_sha=sha256:abc... remote_sha=sha256:def...
🌐 HTTP server starting addr=:8080
```

## Troubleshooting

### Pods Not Being Detected

1. Check RBAC permissions:
```bash
kubectl auth can-i list pods --as=system:serviceaccount:buoy-system:buoy
kubectl auth can-i patch deployments --as=system:serviceaccount:buoy-system:buoy
```

2. Verify annotations are correct:
```bash
kubectl get deployment my-app -o jsonpath='{.metadata.annotations}'
```

### Updates Not Triggering

1. Ensure `imagePullPolicy: Always` is set on your containers
2. Check Buoy logs for errors
3. Verify the image exists in the registry
4. Check if the resource requires approval (check web UI)

### Web UI Not Loading

1. Check ingress configuration:
```bash
kubectl get ingress -n buoy-system
kubectl describe ingress buoy -n buoy-system
```

2. Verify service is running:
```bash
kubectl get svc -n buoy-system
kubectl get endpoints -n buoy-system
```

3. Check pod logs for HTTP server errors

## Uninstalling

```bash
kubectl delete -f deploy/ingress.yaml
kubectl delete -f deploy/service.yaml
kubectl delete -f deploy/deployment.yaml
kubectl delete -f deploy/rbac.yaml
kubectl delete namespace buoy-system
```

## Security Considerations

1. **RBAC**: Buoy requires cluster-wide read access and patch permissions for workloads
2. **Authentication**: Always use forward-auth in production (Traefik + Keycloak)
3. **Network Policies**: Consider restricting Buoy's network access
4. **Image Scanning**: Scan the Buoy image before deployment
5. **Secrets**: Buoy doesn't require any secrets, but ensure your registry credentials are properly configured

## Advanced Configuration

### Custom Port

Change the port in `main.go`:

```go
server.Start(ctx, ":9090")  // Change from :8080
```

Update the deployment and service manifests accordingly.

### Custom Check Intervals

Modify `scheduler.go` ticker durations:

```go
hourlyTicker := time.NewTicker(30 * time.Minute)  // Change from 1 hour
dailyTicker := time.NewTicker(12 * time.Hour)     // Change from 24 hours
```

## Support

For issues, questions, or contributions, please refer to the main README.md file.
