# k8s

K3s manifests for deploying the maritime-flow analytical stack on
Minisforum CachyOS. Runs Spark via the Spark Operator and PostGIS
as a StatefulSet.

This is a run-on-demand deployment — K3s and the Spark cluster are
started when analysis is wanted, not kept running continuously.
The always-on infrastructure (Redpanda, Mosquitto, DuckDNS) lives
on the Pi 4, not here.

---

## What runs here

```
Minisforum (K3s):
  Spark Operator          manages Spark application lifecycle
  PostGIS StatefulSet     spatial reference layers, vessel state
  ais-analyser            Spark job submitted via SparkApplication CR

Pi 4 (systemd, not K3s):
  Redpanda                Kafka-compatible broker, raw NMEA archive
  Mosquitto               MQTT broker, field node command/control
  ais-collector-nmea      Go UDP listener → Redpanda
  Wireguard               VPN gateway
  DuckDNS                 dynamic DNS client
```

Redpanda runs as a native systemd service on the Pi 4, not in K3s.
This follows Redpanda's own recommendation for single-node deployments
and avoids the complexity of persistent volumes for a broker that
needs to survive K3s restarts.

---

## Prerequisites

```bash
# K3s installed on Minisforum
curl -sfL https://get.k3s.io | sh -

# Helm for Spark Operator
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# kubectl configured
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
```

---

## Spark Operator

The Spark Operator manages Spark jobs as Kubernetes custom resources
(`SparkApplication`). Install via Helm:

```bash
helm repo add spark-operator https://kubeflow.github.io/spark-operator
helm repo update
helm install spark-operator spark-operator/spark-operator \
  --namespace spark-operator \
  --create-namespace \
  --set webhook.enable=true
```

---

## Docker image

Do NOT use the `apache/sedona` image — it bundles its own Spark
master/worker and clashes with the Spark Operator.

Build from `apache/spark` with Sedona layered on top:

```dockerfile
FROM apache/spark:3.4.1

ENV SEDONA_PKGS="org.apache.sedona:sedona-spark-shaded-3.4_2.12:1.6.0,\
org.datasyslab:geotools-wrapper:1.6.0-28.2"

RUN ${SPARK_HOME}/bin/spark-shell \
    --packages ${SEDONA_PKGS} \
    --repositories https://repo1.maven.org/maven2 \
    --conf spark.jars.ivy=/tmp/ivy_cache < /dev/null
```

Build and push to a local registry or use as a local image:

```bash
docker build -t maritime-spark-sedona:latest .
# For K3s local image (no registry needed):
docker save maritime-spark-sedona:latest | \
  k3s ctr images import -
```

---

## PostGIS StatefulSet

PostGIS runs as a K3s StatefulSet with a persistent volume for
the reference data:

```bash
kubectl apply -f manifests/postgis-statefulset.yaml
kubectl apply -f manifests/postgis-service.yaml
```

PostGIS is accessed by ais-analyser via JDBC at the service
ClusterIP. The Pi 4 can also reach it via Wireguard for QGIS
editing sessions.

---

## Running ais-analyser

Submit a Spark job via SparkApplication custom resource:

```bash
kubectl apply -f manifests/ais-analyser-job.yaml
```

Monitor:

```bash
kubectl get sparkapplications
kubectl logs -l spark-role=driver -f
```

The job reads from Pi 4 Redpanda (reachable via local network),
processes accumulated NMEA, and writes to Delta Lake on Minisforum
local storage.

---

## Manifest structure

```
k8s/
├── manifests/
│   ├── postgis-statefulset.yaml    PostGIS StatefulSet + PVC
│   ├── postgis-service.yaml        ClusterIP service
│   ├── ais-analyser-job.yaml       SparkApplication CR
│   └── spark-rbac.yaml             RBAC for Spark Operator
├── docker/
│   └── Dockerfile                  apache/spark + Sedona image
└── README.md
```

---

## Updating from Flink to Spark

This module previously contained Flink cluster manifests for the
original Hetzner-based architecture. Those have been removed.
Spark Operator replaces Flink entirely for maritime-flow.

The air-cargo project continues to use Flink — see
[air-cargo/k8s](../../air-cargo/k8s) for those manifests.

---

## Current status

```
K3s on Minisforum          ⬜ not yet installed
Spark Operator             ⬜ not yet installed
Docker image               ⬜ not yet built
PostGIS StatefulSet        ⬜ manifests not yet written
ais-analyser SparkApplication ⬜ manifest not yet written
```
