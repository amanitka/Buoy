# ⚓ Buoy

**Buoy** is a lightweight Kubernetes controller that monitors container image digests and triggers rolling updates as soon as a new version is available in your registry.

If your cluster relies on mutable tags (like `:latest`) or frequently updated staging images, Buoy ensures your workloads stay synchronized with your registry—without requiring manual redeploys or CI/CD overhead.

---

## 🚀 Why Buoy?

Standard Kubernetes behavior won't automatically redeploy a workload when the image behind a tag changes, even with `imagePullPolicy: Always`. This creates a "stale state" where your registry and cluster are out of sync.

Buoy closes that gap by:
* 🔍 **Monitoring** image digests directly from your container registries.
* 🔄 **Detecting** when a tag (e.g., `:stable`) points to a new underlying SHA.
* 🚀 **Triggering** an automated, native rollout for your Deployments and StatefulSets.

---

## ⚙️ How It Works

Buoy operates as a sidecar or standalone controller within your cluster:

1.  **Discovery:** It watches Kubernetes resources (Deployments, StatefulSets) tagged with the Buoy annotation.
2.  **Resolution:** It extracts the image reference (e.g., `my-app:latest`) and resolves its current digest from the registry.
3.  **Comparison:** It compares the registry digest against the last seen digest stored in the resource metadata.
4.  **Action:** If a change is detected, Buoy patches the pod template (typically via a timestamp annotation), prompting Kubernetes to execute a standard rolling update.

---

## 🧩 Quick Start

To enable Buoy for a workload, simply add the `buoy.sh/watch` annotation to your manifest.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  annotations:
    buoy.sh/watch: "true" # ⚓ Buoy starts watching this resource
spec:
  replicas: 3
  template:
    metadata:
      annotations:
        # Buoy will update a timestamp here to trigger the rollout
    spec:
      containers:
        - name: app
          image: my-repo/my-app:latest
          imagePullPolicy: Always