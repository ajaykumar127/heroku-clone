package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Deploy applies a Deployment, Service, and Ingress for the app using kubectl apply -f -.
// Returns combined kubectl output as a string.
func Deploy(cfg *Config, job Job, replicas int) (string, error) {
	var out bytes.Buffer

	namespace := cfg.AppsNamespace
	appName := job.AppName
	image := job.ImageName
	port := 8080

	fmt.Fprintf(&out, "=====> Deploying application: %s\n", appName)
	fmt.Fprintf(&out, "       Image:     %s\n", image)
	fmt.Fprintf(&out, "       Replicas:  %d\n", replicas)
	fmt.Fprintf(&out, "       Namespace: %s\n\n", namespace)

	// Ensure namespace exists
	nsCmd := exec.Command("kubectl", "create", "namespace", namespace, "--dry-run=client", "-o", "yaml")
	nsOut, nsErr := nsCmd.Output()
	if nsErr == nil {
		applyNS := exec.Command("kubectl", "apply", "-f", "-")
		applyNS.Stdin = bytes.NewReader(nsOut)
		applyNS.Stdout = &out
		applyNS.Stderr = &out
		_ = applyNS.Run() // non-fatal if namespace already exists
	}

	// Build the K8s manifest inline
	manifest := buildManifest(appName, image, namespace, port, replicas)

	fmt.Fprintf(&out, "-----> Applying Kubernetes manifests\n")

	// kubectl apply -f -
	applyCmd := exec.Command("kubectl", "apply", "-f", "-")
	applyCmd.Stdin = strings.NewReader(manifest)
	applyCmd.Stdout = &out
	applyCmd.Stderr = &out
	if err := applyCmd.Run(); err != nil {
		return out.String(), fmt.Errorf("kubectl apply: %w", err)
	}

	// Wait for rollout
	fmt.Fprintf(&out, "\n-----> Waiting for rollout to complete\n")
	rolloutCmd := exec.Command(
		"kubectl", "rollout", "status",
		fmt.Sprintf("deployment/%s", appName),
		"-n", namespace,
		"--timeout=5m",
	)
	rolloutCmd.Stdout = &out
	rolloutCmd.Stderr = &out
	if err := rolloutCmd.Run(); err != nil {
		return out.String(), fmt.Errorf("rollout status: %w", err)
	}

	fmt.Fprintf(&out, "\n=====> Deployment complete!\n")
	fmt.Fprintf(&out, "       URL: http://%s.%s\n", appName, namespace)
	return out.String(), nil
}

// buildManifest generates the YAML for a Deployment, Service, and Ingress.
func buildManifest(appName, image, namespace string, port, replicas int) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s
  namespace: %[3]s
  labels:
    app: %[1]s
spec:
  replicas: %[5]d
  selector:
    matchLabels:
      app: %[1]s
  template:
    metadata:
      labels:
        app: %[1]s
    spec:
      containers:
      - name: web
        image: %[2]s
        ports:
        - containerPort: %[4]d
          name: http
        env:
        - name: PORT
          value: "%[4]d"
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
  name: %[1]s
  namespace: %[3]s
  labels:
    app: %[1]s
spec:
  type: ClusterIP
  ports:
  - port: 80
    targetPort: http
    protocol: TCP
    name: http
  selector:
    app: %[1]s
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %[1]s
  namespace: %[3]s
  labels:
    app: %[1]s
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
  - host: %[1]s.localhost
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: %[1]s
            port:
              number: 80
`, appName, image, namespace, port, replicas)
}
