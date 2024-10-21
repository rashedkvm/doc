package dist
apiVersion: v1
kind: Namespace
metadata:
name: crdsdev
---
apiVersion: v1
kind: ConfigMap
metadata:
name: doc-repos
namespace: crdsdev
data:
repos: |
crossplane/crossplane,
crossplane/provider-alibaba,
crossplane/provider-aws,
crossplane/provider-azure,
crossplane/provider-gcp,
crossplane/provider-rook,
crossplane/oam-kubernetes-runtime,
crossplane-contrib/provider-helm,
jetstack/cert-manager,
kubernetes-sigs/cluster-api,
kubernetes-sigs/cluster-api-provider-packet,
kubernetes-sigs/security-profiles-operator,
packethost/crossplane-provider-packet,
projectcontour/contour,
schemahero/schemahero
---
apiVersion: apps/v1
kind: Deployment
metadata:
name: doc
namespace: crdsdev
labels:
app: doc
spec:
selector:
matchLabels:
app: doc
template:
metadata:
labels:
app: doc
spec:
containers:
- name: doc
image: ko://github.com/crdsdev/doc/cmd/doc
env:
- name: REDIS_HOST
valueFrom:
secretKeyRef:
name: doc-redis
key: endpoint
- name: ANALYTICS
value: "true"
ports:
- containerPort: 5000
name: doc
---
apiVersion: v1
kind: Service
metadata:
name: doc
namespace: crdsdev
labels:
app: doc
spec:
ports:
- port: 80
targetPort: 5000
selector:
app: doc
type: NodePort
---
apiVersion: batch/v1beta1
kind: CronJob
metadata:
name: gitter
namespace: crdsdev
labels:
app: doc
spec:
schedule: "@hourly"
jobTemplate:
spec:
backoffLimit: 0
template:
spec:
containers:
- name: gitter
image: ko://github.com/crdsdev/doc/cmd/gitter
env:
- name: REDIS_HOST
valueFrom:
secretKeyRef:
name: doc-redis
key: endpoint
- name: REPOS
valueFrom:
configMapKeyRef:
name: doc-repos
key: repos
restartPolicy: Never