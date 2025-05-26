# Kubernetes Chart and Manifests

The chart files and kubernetes manifests are generated from the templates present in `/kubernetes/templates/chart/`.

Any required changes should be made in the templates and not in these files.

## Helm Chart from OCI Registry

The Helm chart for GitWebhookProxy is published to Docker Hub OCI registry.

**OCI Registry Path:** `oci://docker.io/maxisam/gitwebhookproxyhelm`

You can use Helm to directly pull or install the chart from this OCI registry.

### Pulling the Chart

To download and inspect the chart without installing it, use the `helm pull` command. Replace `[CHART_VERSION]` with the specific version you want to pull.

```bash
helm pull oci://docker.io/maxisam/gitwebhookproxyhelm/gitwebhookproxy --version [CHART_VERSION]
```

This will download the chart package (e.g., `gitwebhookproxy-[CHART_VERSION].tgz`) to your current directory.

### Installing the Chart

To install the chart directly from the OCI registry into your Kubernetes cluster, use the `helm install` command. Replace `my-release` with your desired release name and `[CHART_VERSION]` with the chart version.

```bash
helm install my-release oci://docker.io/maxisam/gitwebhookproxyhelm/gitwebhookproxy --version [CHART_VERSION]
```

You can find available chart versions on the [GitHub Releases page](https://github.com/maxisam/GitWebhookProxy/releases) (look for tags like `helm-vX.Y.Z`).