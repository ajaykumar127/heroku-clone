const http = require('http');

const PORT = process.env.PORT || 8080;
const VERSION = process.env.VERSION || 'v1';

const server = http.createServer((req, res) => {
  console.log(`${new Date().toISOString()} ${req.method} ${req.url}`);

  if (req.url === '/health') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok', version: VERSION }));
    return;
  }

  res.writeHead(200, { 'Content-Type': 'text/html' });
  res.end(`
    <!DOCTYPE html>
    <html>
      <head>
        <title>Platform POC</title>
        <style>
          body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
          }
          .container {
            background: rgba(255, 255, 255, 0.1);
            border-radius: 10px;
            padding: 40px;
            backdrop-filter: blur(10px);
          }
          h1 { margin-top: 0; }
          .info {
            background: rgba(0, 0, 0, 0.2);
            padding: 15px;
            border-radius: 5px;
            margin: 20px 0;
          }
          code {
            background: rgba(0, 0, 0, 0.3);
            padding: 2px 6px;
            border-radius: 3px;
          }
        </style>
      </head>
      <body>
        <div class="container">
          <h1>🚀 Platform POC - Success!</h1>
          <p>Your application is running on the cloud-agnostic PaaS platform.</p>

          <div class="info">
            <strong>Application Info:</strong><br>
            Version: <code>${VERSION}</code><br>
            Process ID: <code>${process.pid}</code><br>
            Node Version: <code>${process.version}</code><br>
            Uptime: <code>${Math.floor(process.uptime())}s</code>
          </div>

          <h2>What just happened?</h2>
          <ol>
            <li>You pushed code via <code>git push</code></li>
            <li>Git server received the push</li>
            <li>Buildpack detected Node.js and built the app</li>
            <li>Container image was created</li>
            <li>Kubernetes deployed your app</li>
            <li>You're seeing this page!</li>
          </ol>

          <h2>Try these commands:</h2>
          <pre><code>curl http://${req.headers.host}/health
kubectl get pods -n apps
kubectl logs -n apps -l app=my-app --tail=20</code></pre>
        </div>
      </body>
    </html>
  `);
});

server.listen(PORT, () => {
  console.log(`Server running on port ${PORT}`);
  console.log(`Version: ${VERSION}`);
  console.log(`Node: ${process.version}`);
});

// Graceful shutdown
process.on('SIGTERM', () => {
  console.log('SIGTERM received, shutting down gracefully...');
  server.close(() => {
    console.log('Server closed');
    process.exit(0);
  });
});
