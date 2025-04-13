# 🔁 Kwatcher Operator

**Kwatcher** is a Kubernetes operator that:

1. **Automatically creates a ConfigMap** from data fetched from an external URL using a secured `Secret`,
2. **Periodically polls** the URL (based on `refreshInterval`),
3. **Updates the ConfigMap** when the data changes,
4. And **automatically triggers pod redeployment** via annotations in the related `Deployments`.

---

## ❓ Why Use Kwatcher

Kwatcher is especially useful when your application relies on **external configuration data** that changes dynamically. It automates the entire flow:

- fetching external data,
- updating a Kubernetes `ConfigMap`,
- and redeploying affected pods.

### ✅ Common Use Cases

- **Dynamic configuration**: feature flags, business rules, or third-party configs.
- **Cross-cluster syncing**: consuming external configuration from other environments or clusters.
- **Auto-sync without CI/CD**: no manual rollouts or pipeline triggers.
- **Fast updates**: reacts to data changes in near real-time (e.g., every 30s).

### ⭐ Benefits

- 100% declarative (Kubernetes-native)
- Secured with `Secret` integration
- Avoids unnecessary redeployments
- Ideal for config-sensitive microservices

---

## ⚙️ How It Works

1. **Define a `Kwatcher` custom resource** with:
   - An external URL,
   - Port number,
   - Polling interval (`refreshInterval`),
   - A `Secret` with credentials.

> **⚠️ Important:**  
> The referenced `Secret` **must exist before** the `Kwatcher` is created.  
> Otherwise, the custom resource will failed.

2. **Automatic ConfigMap creation**

   When a `Kwatcher` is created, the operator generates a `ConfigMap` with the **same name** (e.g., `example-kwatcher`) and stores the fetched content in it. It is refreshed at the specified interval.

3. **Triggering pod redeployment**

   In your `Deployment`, add annotations like:
   ```yaml
   kwatcher.config/watched-configmaps: "example-kwatcher"
   kwatcher.config/update-policy: "explicit"
   ```
   The operator will patch an additional annotation ( `kwatcher.config/last-updated`) on the pod template to trigger Kubernetes to roll out a new version.

---

## 📄 Example: Custom Resource

```yaml
apiVersion: core.kwatch.cloudcorner.org/v1beta1
kind: Kwatcher
metadata:
  name: example-kwatcher
spec:
  provider:
    port: 443
    url: "https://api.jsonbin.io/v3/b/67efdc378960c979a57e2d50"
  config:
    refreshInterval: 30
    secret: "secret-1"
```

---

## 🔐 Required Secret

> 🧾 **How headers work**
>
> - `client-key`: the actual API key to be used.
> - `key-type`: the HTTP header name (e.g. `Authorization`, `X-API-Key`) in which the key must be sent.
>
> Example: if `key-type` is `X-Access-Key`, then the request will include:
>


```yaml
apiVersion: v1
kind: Secret
metadata:
  name: secret-1
type: Opaque
data:
  client-key: <base64 encoded value>
  key-type: <base64 encoded value> # "Authorization/X-API-Key/.."
```

> The `Secret` must already exist before applying the `Kwatcher`.

---

## 🧪 Example: Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 4
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
      annotations:
        redeploy-check: "initial"
        kwatcher.config/update-policy: "explicit"
        kwatcher.config/watched-configmaps: "example-kwatcher"
    spec:
      containers:
        - name: nginx
          image: nginx:1.25
          ports:
            - containerPort: 80
```

---

## 📌 Architecture Overview

```
+-----------------------------+
|  Secret (credentials)       |
|  must exist BEFORE the      |
|  Kwatcher is created         |
+-------------+---------------+
              |
              v
+-----------------------------+
|  Custom Resource            |
|  kind: Kwatcher             |
|  (url + secret + interval) |
+-------------+---------------+
              |
              v
+-----------------------------+
|  Periodic HTTP call         |
|  to external URL (e.g. 30s) |
+-------------+---------------+
              |
              v
+-----------------------------+
|  ConfigMap (auto-created)   |
|  same name as the CR        |
+-------------+---------------+
              |
              v
+-----------------------------+
|  Deployment with annotations|
|  - kwatcher.config/...      |
|  - watched-configmaps       |
+-------------+---------------+
              |
              v
+-----------------------------+
|  Operator patches annotation|
|  -> triggers redeployment   |
+-----------------------------+
```

---

## ⚙️ Generate and Deploy the Operator with Helm

To fully generate and deploy the operator using Helm, you can run the following command:

```bash
make all
```

This command performs the entire workflow:

1. **Generates Kubernetes manifests** using `controller-gen` (via `make manifests`)
2. **Builds the Docker image** for the operator
3. **Pushes the image** to your Docker registry
4. **Installs required tools**: `kustomize` and `helmify` if missing
5. **Generates a Helm chart** from Kustomize output using `helmify`
6. **Updates the Helm chart** with the correct Docker image and tag
7. **Packages the chart** and updates the local Helm repo index

---

### 🚀 To install the generated Helm chart on your cluster:

```bash
helm install kwatcher ./charts/kwatcher-operator-<version>.tgz
```

Replace `<version>` with the appropriate chart version (e.g. `0.1.0`).

## 🚀 Install via Helm (from GitHub Container Registry)

To install the operator directly from GitHub Container Registry (GHCR), use the following command:

```bash
helm install kwatcher oci://ghcr.io/berg-it/kwatcher-operator --version 0.1.0
```
This will install the Helm chart named `kwatcher-operator` from my public GHCR package.

## 🛠️ Roadmap

- [ ] Support multiple URLs per `Kwatcher`
- [ ] Advanced update strategies (e.g. rolling, blue/green)
- [ ] Schema validation of external data

---

## 🤝 Contributing

Contributions are welcome!  
Feel free to open an issue or submit a pull request to improve the project.

---

## 📝 License

Distributed under the MIT License.
