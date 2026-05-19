package main

import (
	"fmt"
	"strings"
)

// DeployRequest is the payload for a deploy operation.
type DeployRequest struct {
	AppName  string            `json:"app_name"`
	Image    string            `json:"image"`
	Replicas int               `json:"replicas"`
	Port     int               `json:"port"`
	EnvVars  map[string]string `json:"env_vars"`
}

// GenerateManifest returns the combined YAML for Deployment + Service + Ingress,
// separated by "---".
func GenerateManifest(cfg *Config, req DeployRequest) string {
	port := req.Port
	if port == 0 {
		port = 8080
	}
	replicas := req.Replicas
	if replicas == 0 {
		replicas = 1
	}

	envBlock := buildEnvBlock(req.EnvVars, port)

	deployment := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
      - name: web
        image: %s
        ports:
        - containerPort: %d
          name: http
%s        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "500m"
            memory: "512Mi"
        livenessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5`,
		req.AppName, cfg.Namespace, req.AppName,
		replicas,
		req.AppName,
		req.AppName,
		req.Image,
		port,
		envBlock,
	)

	service := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  type: ClusterIP
  ports:
  - port: 80
    targetPort: http
    protocol: TCP
    name: http
  selector:
    app: %s`,
		req.AppName, cfg.Namespace, req.AppName, req.AppName,
	)

	ingress := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: %s
  rules:
  - host: %s.%s
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: %s
            port:
              number: 80`,
		req.AppName, cfg.Namespace, req.AppName,
		cfg.IngressClass,
		req.AppName, cfg.AppsDomain,
		req.AppName,
	)

	return strings.Join([]string{deployment, service, ingress}, "\n---\n")
}

// buildEnvBlock generates the YAML env section for the container, indented for
// embedding inside the container spec. If there are no env vars, just PORT is
// added. The returned string includes trailing newline so the caller can
// concatenate directly.
func buildEnvBlock(envVars map[string]string, port int) string {
	var sb strings.Builder
	sb.WriteString("        env:\n")
	sb.WriteString(fmt.Sprintf("        - name: PORT\n          value: \"%d\"\n", port))
	for k, v := range envVars {
		sb.WriteString(fmt.Sprintf("        - name: %s\n          value: %q\n", k, v))
	}
	return sb.String()
}
