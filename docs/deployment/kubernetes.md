# Deploying SkyEye on Kubernetes

This guide deploys SkyEye on a Kubernetes cluster with the manifests in [`init/kubernetes/`](../../init/kubernetes/), optionally together with the [autoscaler](../ADMIN.md#autoscaling-experimental) so SkyEye only runs while players are on its frequencies. It assumes you already run a cluster and are comfortable with `kubectl`.

By default the manifests use **local speech recognition** (whisper.cpp on the CPU) and download the speech recognition model automatically. [Cloud speech recognition](#cloud-speech-recognition) and [GPU speech recognition](#gpu-speech-recognition) are covered at the end.

## Requirements

- A node with a fast CPU that supports [AVX2](https://en.wikipedia.org/wiki/Advanced_Vector_Extensions#Advanced_Vector_Extensions_2), at least 4 real (not shared-VM) cores that are not kept busy by other workloads, and 3 GiB of memory for SkyEye. This is the same hardware guidance as in [Planning Your Deployment](../ADMIN.md#planning-your-deployment); shared or heavily contended cores cause high latency and stuttering audio.
- A default StorageClass (or set `storageClassName` in `models.yaml`) for a 2 GiB volume that caches the model.
- Outbound access from the cluster to your SRS server, your Tacview real-time telemetry server and `huggingface.co` (first start only).

## What gets deployed

All objects go into the namespace `skyeye` (change `namespace:` in `kustomization.yaml` to use another one).

| Object | Purpose |
| --- | --- |
| Deployment `skyeye` | SkyEye itself. Reads `/etc/skyeye/config.yaml` from the ConfigMap `skyeye-config`. Uses `strategy: Recreate` so two bots with the same callsign never overlap. |
| Init container `fetch-model` | Before SkyEye starts, downloads the model named by `whisper-model` in `config.yaml` from [Hugging Face](https://huggingface.co/ggerganov/whisper.cpp) into the volume, unless it is already there. |
| PersistentVolumeClaim `skyeye-models` | Caches the model (`ggml-small.en.bin` is about 470 MiB), so restarts and autoscaling do not download it again. |
| ConfigMap `skyeye-config` | Generated from [`config.yaml`](../../init/kubernetes/config.yaml) (plus any custom aircraft/locations files you add). A hash suffix in the name restarts SkyEye when the file changes. |
| Secret `skyeye` (you create it) | Passwords and API keys as `SKYEYE_*` environment variables. Optional. |
| Deployment `skyeye-scaler` (optional) | The autoscaler plus a small webhook sidecar ([`skyeye-scaler-webhook.py`](../../init/kubernetes/skyeye-scaler-webhook.py)) that scales the `skyeye` Deployment between 0 and 1. |
| ServiceAccount/Role/RoleBinding `skyeye-scaler` | Lets the webhook `get`/`patch` the scale of the `skyeye` Deployment, and nothing else. |

Nothing listens for inbound connections: SkyEye and the scaler only connect out. No Service or Ingress is needed.

## Configure

Copy `init/kubernetes/` somewhere you keep your cluster configuration, then:

1. Edit `config.yaml`: set `telemetry-address`, `srs-server-address`, `srs-frequencies`, `callsign` and any other option from the [sample config file](../../config.yaml). Leave passwords out of this file.
2. Choose the model with `whisper-model`. It must be a path under `/models/` and the file name must exist in the [whisper.cpp model repository](https://huggingface.co/ggerganov/whisper.cpp/tree/main): `ggml-small.en.bin` (default, recommended), `ggml-medium.en.bin` (better quality, needs a powerful CPU, more memory and a larger volume) or `ggml-tiny.en.bin` (only if the small model is too slow). Changing it downloads the new model on the next start.
3. Check the resources of the `skyeye` container in `skyeye.yaml`. The defaults request 1 CPU and 3 GiB of memory (4 GiB limit) and set no CPU limit, like the upstream container setup. whisper.cpp starts one thread per CPU it can see, which in a pod is every core of the node, so the CPU request does not change how many cores a transcription uses; it only sets SkyEye's share of CPU time when the node is busy. On a contended node, raise the request (or use the kubelet's [static CPU manager policy](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/) with equal integer requests and limits to pin dedicated cores). Do not add a CPU limit on its own: throttling makes recognition stutter. To keep SkyEye on your fastest node, add a `nodeSelector` or node affinity.
4. If you use the scaler, set the same `--srs-server-address` and `--srs-frequencies` in `scaler.yaml`. If you don't, remove `scaler.yaml` from `kustomization.yaml`.
5. Pin image versions in the `images:` section of `kustomization.yaml` (see the [releases page](https://github.com/dharmab/skyeye/releases)).

   The manifests pull `ghcr.io/dharmab/skyeye` and `ghcr.io/dharmab/skyeye-scaler`. The scaler needs a release that includes the fixes for its start-up check, its first-call `stop` and its shutdown hang; older `skyeye-scaler` images exit immediately with these flags. Until such a release exists, build the scaler image from this repository and point `kustomization.yaml` at it:

   ```sh
   docker build --target skyeye-scaler -t registry.example.com/skyeye-scaler:dev .
   docker push registry.example.com/skyeye-scaler:dev
   ```

   ```yaml
   images:
     - name: ghcr.io/dharmab/skyeye-scaler
       newName: registry.example.com/skyeye-scaler
       newTag: dev
   ```

6. Create the Secret with the values you need (any [option](../ADMIN.md#configuration) can be set as `SKYEYE_<OPTION>`, with dashes replaced by underscores):

   ```sh
   kubectl create namespace skyeye
   kubectl -n skyeye create secret generic skyeye \
     --from-literal=SKYEYE_SRS_EAM_PASSWORD=... \
     --from-literal=SKYEYE_TELEMETRY_PASSWORD=...
   ```

   The scaler reads the EAM password from the same Secret.

To add custom aircraft or locations, put the files next to `config.yaml`, list them under `configMapGenerator` in `kustomization.yaml`, and set `aircraft-file: /etc/skyeye/custom-aircraft.yaml` or `locations-file: /etc/skyeye/locations.yaml` in `config.yaml`.

## Deploy

```sh
kubectl apply -k path/to/your/copy
kubectl -n skyeye get deploy,pods
# First start: watch the model download, then SkyEye loading it
kubectl -n skyeye logs deploy/skyeye -c fetch-model
kubectl -n skyeye logs deploy/skyeye -c skyeye
```

SkyEye is ready when its log shows `whisper model loaded` and `SRS client starting`. Re-run `kubectl apply -k` after any change. The `skyeye` Deployment has no `replicas` field on purpose, so applying the manifests never overrides the scaler's decision.

## How the scaler decides

The scaler joins SRS as `SkyEye Scaler <id> [BOT]` on SkyEye's frequencies and every minute counts the human players tuned to one of them (clients whose name ends in `[BOT]` are ignored). It sends the result to the webhook sidecar, which:

| Situation | SkyEye |
| --- | --- |
| A player tunes one of the frequencies | Scaled to 1 within about a minute, plus the time to load the model |
| Nobody on the frequencies for `--stop-delay` (10 minutes) | Scaled to 0 |
| Players dropped off less than 10 minutes ago, or the scaler just restarted | No change |
| The scaler cannot reach SRS for `WATCHDOG_SECONDS` (5 minutes) | Scaled to 0 |

On a fresh install SkyEye starts with 1 replica and is stopped after the first 10 minutes without players.

The scaler exits when it cannot reach SRS, so `scaler.yaml` runs it in a shell loop that restarts it every 30 seconds instead of letting the container crash-loop.

Useful commands:

```sh
# Scale decisions (one JSON line per change)
kubectl -n skyeye logs deploy/skyeye-scaler -c webhook
# What the scaler sees
kubectl -n skyeye logs deploy/skyeye-scaler -c scaler
# Force SkyEye on for testing: pause the scaler first
kubectl -n skyeye scale deploy/skyeye-scaler --replicas=0
kubectl -n skyeye scale deploy/skyeye --replicas=1
```

## Cloud speech recognition

To use the OpenAI Whisper API instead of a local model, uncomment the `components:` block in `kustomization.yaml`:

```yaml
components:
  - components/openai-api
```

The [component](../../init/kubernetes/components/openai-api/kustomization.yaml) removes the model volume and the download, sets `SKYEYE_RECOGNIZER=openai-whisper-api` (environment variables override `config.yaml`) and lowers the resources to 100m CPU and 256 MiB of memory. Add your key to the Secret:

```sh
kubectl -n skyeye create secret generic skyeye \
  --from-literal=SKYEYE_OPENAI_API_KEY=sk-... \
  --from-literal=SKYEYE_SRS_EAM_PASSWORD=... \
  --from-literal=SKYEYE_TELEMETRY_PASSWORD=...
```

If you switch an existing installation, delete the PVC afterwards: `kubectl -n skyeye delete pvc skyeye-models`.

## GPU speech recognition

The Vulkan build runs whisper.cpp on a GPU (see the [Linux GPU guide](linux/gpu.md) for supported hardware). To adapt the manifests:

- Use the `-vulkan` image tag, e.g. `newTag: v1.10.1-vulkan` for `ghcr.io/dharmab/skyeye` in `kustomization.yaml`.
- Give the `skyeye` container the GPU through your cluster's device plugin, e.g. `resources.limits: {nvidia.com/gpu: 1}` with the NVIDIA device plugin, or `gpu.intel.com/i915: 1` / `amd.com/gpu: 1` for Intel and AMD.
- Lower the CPU request; recognition no longer needs dedicated cores.
