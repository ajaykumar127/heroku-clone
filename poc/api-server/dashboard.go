package main

import (
	"html/template"
	"net/http"
)

const dashboardHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>Platform Dashboard</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: #333;
            min-height: 100vh;
            padding: 20px;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        header {
            background: rgba(255, 255, 255, 0.95);
            padding: 30px;
            border-radius: 10px;
            margin-bottom: 30px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        h1 {
            color: #667eea;
            margin-bottom: 10px;
        }
        .subtitle {
            color: #666;
            font-size: 14px;
        }
        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .stat-card {
            background: rgba(255, 255, 255, 0.95);
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        .stat-label {
            font-size: 12px;
            color: #666;
            text-transform: uppercase;
            letter-spacing: 1px;
            margin-bottom: 8px;
        }
        .stat-value {
            font-size: 32px;
            font-weight: bold;
            color: #667eea;
        }
        .apps-section {
            background: rgba(255, 255, 255, 0.95);
            padding: 30px;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        .apps-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
        }
        h2 {
            color: #333;
        }
        button {
            background: #667eea;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 5px;
            cursor: pointer;
            font-size: 14px;
            transition: background 0.3s;
        }
        button:hover {
            background: #5568d3;
        }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th {
            text-align: left;
            padding: 12px;
            background: #f8f9fa;
            color: #666;
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        td {
            padding: 15px 12px;
            border-bottom: 1px solid #eee;
        }
        tr:last-child td {
            border-bottom: none;
        }
        .app-name {
            font-weight: 600;
            color: #667eea;
        }
        .app-url {
            color: #666;
            font-size: 14px;
        }
        .app-url a {
            color: #667eea;
            text-decoration: none;
        }
        .app-url a:hover {
            text-decoration: underline;
        }
        .empty-state {
            text-align: center;
            padding: 60px 20px;
            color: #999;
        }
        .empty-state-icon {
            font-size: 48px;
            margin-bottom: 16px;
        }
        .code {
            background: #2d3748;
            color: #68d391;
            padding: 20px;
            border-radius: 5px;
            margin-top: 20px;
            font-family: 'Courier New', monospace;
            font-size: 13px;
            overflow-x: auto;
        }
        .quick-start {
            background: rgba(255, 255, 255, 0.95);
            padding: 30px;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
            margin-top: 30px;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>🚀 Platform Dashboard</h1>
            <p class="subtitle">Cloud-Agnostic PaaS - Proof of Concept</p>
        </header>

        <div class="stats">
            <div class="stat-card">
                <div class="stat-label">Total Apps</div>
                <div class="stat-value" id="total-apps">-</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Total Releases</div>
                <div class="stat-value" id="total-releases">-</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">API Status</div>
                <div class="stat-value" style="color: #48bb78;">✓ OK</div>
            </div>
        </div>

        <div class="apps-section">
            <div class="apps-header">
                <h2>Applications</h2>
                <button onclick="refreshApps()">Refresh</button>
            </div>
            <div id="apps-content">
                <div class="empty-state">
                    <div class="empty-state-icon">📦</div>
                    <p>Loading applications...</p>
                </div>
            </div>
        </div>

        <div class="quick-start">
            <h2>Quick Start</h2>
            <p style="margin: 15px 0;">Create and deploy your first app:</p>
            <div class="code">
# Create an app<br>
curl -X POST http://localhost:8080/v1/apps -d '{"name":"my-app"}'<br>
<br>
# Deploy example app<br>
cd example-app<br>
git init && git add . && git commit -m "Initial commit"<br>
git remote add platform ssh://git@localhost:2222/my-app.git<br>
git push platform main
            </div>
        </div>
    </div>

    <script>
        async function loadApps() {
            try {
                const response = await fetch('/v1/apps');
                const apps = await response.json();

                document.getElementById('total-apps').textContent = apps.length;

                if (apps.length === 0) {
                    document.getElementById('apps-content').innerHTML =
                        '<div class="empty-state">' +
                        '<div class="empty-state-icon">&#128230;</div>' +
                        '<p>No applications yet</p>' +
                        '<p style="margin-top:10px;font-size:14px;">Create your first app using the API or CLI</p>' +
                        '</div>';
                } else {
                    let totalReleases = 0;
                    const tableRows = await Promise.all(apps.map(async function(app) {
                        const relResponse = await fetch('/v1/apps/' + app.name + '/releases');
                        const releases = await relResponse.json();
                        totalReleases += releases.length;
                        return '<tr>' +
                            '<td><div class="app-name">' + app.name + '</div></td>' +
                            '<td><div class="app-url"><a href="' + app.web_url + '" target="_blank">' + app.web_url + '</a></div></td>' +
                            '<td><div class="app-url">' + app.git_url + '</div></td>' +
                            '<td>' + releases.length + ' releases</td>' +
                            '<td>' + new Date(app.created_at).toLocaleDateString() + '</td>' +
                            '</tr>';
                    }));

                    document.getElementById('total-releases').textContent = totalReleases;
                    document.getElementById('apps-content').innerHTML =
                        '<table><thead><tr>' +
                        '<th>Name</th><th>Web URL</th><th>Git URL</th><th>Releases</th><th>Created</th>' +
                        '</tr></thead><tbody>' + tableRows.join('') + '</tbody></table>';
                }
            } catch (error) {
                document.getElementById('apps-content').innerHTML =
                    '<div class="empty-state">' +
                    '<div class="empty-state-icon">&#9888;</div>' +
                    '<p>Failed to load applications</p>' +
                    '<p style="margin-top:10px;font-size:14px;color:#e53e3e;">' + error.message + '</p>' +
                    '</div>';
            }
        }

        function refreshApps() { loadApps(); }

        loadApps();
        setInterval(loadApps, 10000);
    </script>
</body>
</html>
`

func (s *APIServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))
	tmpl.Execute(w, nil)
}
