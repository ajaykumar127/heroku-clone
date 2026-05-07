#!/bin/bash
# Deploy application to Kubernetes
# Generates K8s manifests and applies them

set -e

APP_NAME=$1
IMAGE=$2
REPLICAS=${3:-1}

if [ -z "$APP_NAME" ] || [ -z "$IMAGE" ]; then
    echo "Usage: $0 <app-name> <image> [replicas]"
    exit 1
fi

NAMESPACE="apps"
PORT=8080

echo "=====> Deploying application: $APP_NAME"
echo "       Image: $IMAGE"
echo "       Replicas: $REPLICAS"
echo "       Namespace: $NAMESPACE"
echo ""

# Create namespace if it doesn't exist
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# Generate Deployment manifest
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${APP_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: ${APP_NAME}
spec:
  replicas: ${REPLICAS}
  selector:
    matchLabels:
      app: ${APP_NAME}
  template:
    metadata:
      labels:
        app: ${APP_NAME}
    spec:
      containers:
      - name: web
        image: ${IMAGE}
        ports:
        - containerPort: ${PORT}
          name: http
        env:
        - name: PORT
          value: "${PORT}"
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: ${APP_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: ${APP_NAME}
spec:
  type: ClusterIP
  ports:
  - port: 80
    targetPort: http
    protocol: TCP
    name: http
  selector:
    app: ${APP_NAME}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ${APP_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: ${APP_NAME}
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
  - host: ${APP_NAME}.localhost
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ${APP_NAME}
            port:
              number: 80
EOF

echo ""
echo "-----> Waiting for deployment to be ready..."
kubectl rollout status deployment/${APP_NAME} -n ${NAMESPACE} --timeout=5m

echo ""
echo "=====> Deployment complete!"
echo "       URL: http://${APP_NAME}.localhost"
echo ""
echo "       Run: curl http://${APP_NAME}.localhost"
echo ""
