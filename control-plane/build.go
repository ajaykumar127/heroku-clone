package main

import (
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"
)

type BuildManager struct {
	db       *sql.DB
	config   *Config
	postgres bool
}

func NewBuildManager(db *sql.DB, config *Config) *BuildManager {
	return &BuildManager{db: db, config: config, postgres: config.IsPostgres()}
}

// ph returns the SQL placeholder for position n (1-based).
func (bm *BuildManager) ph(n int) string {
	return placeholder(bm.postgres, n)
}

// TriggerBuild creates a release and either dispatches to a runtime agent (via a Job)
// or runs the local pipeline inline if no runtimes are registered.
// Returns (releaseID, jobID, error). jobID is empty when using the local pipeline.
func (bm *BuildManager) TriggerBuild(appName, commit string) (string, string, error) {
	// Look up the app.
	var appID string
	var preferredRuntimeID sql.NullString
	err := bm.db.QueryRow(
		fmt.Sprintf("SELECT id, runtime_id FROM apps WHERE name = %s", bm.ph(1)), appName,
	).Scan(&appID, &preferredRuntimeID)
	if err != nil {
		return "", "", fmt.Errorf("app %q not found: %w", appName, err)
	}

	// Determine which runtime to use.
	runtimeID := ""
	if preferredRuntimeID.Valid && preferredRuntimeID.String != "" {
		// Use the app's preferred runtime if it is still active.
		var status string
		e := bm.db.QueryRow(
			fmt.Sprintf("SELECT status FROM runtimes WHERE id = %s", bm.ph(1)),
			preferredRuntimeID.String,
		).Scan(&status)
		if e == nil && status == "active" {
			runtimeID = preferredRuntimeID.String
		}
	}

	if runtimeID == "" {
		// Pick the first active runtime.
		e := bm.db.QueryRow(
			"SELECT id FROM runtimes WHERE status = 'active' ORDER BY registered_at ASC LIMIT 1",
		).Scan(&runtimeID)
		if e != nil {
			runtimeID = "" // No runtimes available — fall back to local builder.
		}
	}

	// Create the release record.
	releaseID, version, err := bm.createRelease(appID, commit, runtimeID)
	if err != nil {
		return "", "", fmt.Errorf("failed to create release: %w", err)
	}

	if runtimeID == "" {
		// No runtime registered — use the local builder pipeline.
		go bm.runPipeline(releaseID, appName, commit, version)
		return releaseID, "", nil
	}

	// Create a Job for the runtime agent.
	jobID, err := bm.createJob(runtimeID, appID, releaseID, appName, commit)
	if err != nil {
		return "", "", fmt.Errorf("failed to create job: %w", err)
	}

	log.Printf("[build] job created app=%s commit=%s release=%s job=%s runtime=%s",
		appName, commit, releaseID, jobID, runtimeID)
	return releaseID, jobID, nil
}

func (bm *BuildManager) createRelease(appID, commit, runtimeID string) (string, int, error) {
	var version int
	err := bm.db.QueryRow(
		fmt.Sprintf("SELECT COALESCE(MAX(version), 0) + 1 FROM releases WHERE app_id = %s", bm.ph(1)),
		appID,
	).Scan(&version)
	if err != nil {
		return "", 0, err
	}

	id := generateID()
	now := time.Now()

	if runtimeID != "" {
		_, err = bm.db.Exec(
			fmt.Sprintf(
				`INSERT INTO releases (id, app_id, version, "commit", status, build_output, runtime_id, created_at)
				 VALUES (%s, %s, %s, %s, 'building', '', %s, %s)`,
				bm.ph(1), bm.ph(2), bm.ph(3), bm.ph(4), bm.ph(5), bm.ph(6),
			),
			id, appID, version, commit, runtimeID, now,
		)
	} else {
		_, err = bm.db.Exec(
			fmt.Sprintf(
				`INSERT INTO releases (id, app_id, version, "commit", status, build_output, created_at)
				 VALUES (%s, %s, %s, %s, 'building', '', %s)`,
				bm.ph(1), bm.ph(2), bm.ph(3), bm.ph(4), bm.ph(5),
			),
			id, appID, version, commit, now,
		)
	}
	return id, version, err
}

func (bm *BuildManager) createJob(runtimeID, appID, releaseID, appName, commit string) (string, error) {
	id := generateID()
	now := time.Now()

	repoURL := fmt.Sprintf("ssh://git@%s:%s/%s.git", bm.config.GitServerHost, bm.config.GitServerPort, appName)
	imageName := fmt.Sprintf("%s/%s:%s", bm.config.RegistryURL, appName, commit)

	_, err := bm.db.Exec(
		fmt.Sprintf(
			`INSERT INTO jobs (id, runtime_id, app_id, release_id, app_name, "commit", repo_url, image_name, type, status, log_output, created_at, updated_at)
			 VALUES (%s, %s, %s, %s, %s, %s, %s, %s, 'build_and_deploy', 'pending', '', %s, %s)`,
			bm.ph(1), bm.ph(2), bm.ph(3), bm.ph(4), bm.ph(5),
			bm.ph(6), bm.ph(7), bm.ph(8), bm.ph(9), bm.ph(10),
		),
		id, runtimeID, appID, releaseID, appName, commit, repoURL, imageName, now, now,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

// runPipeline is used when no runtime agent is registered (local fallback).
func (bm *BuildManager) runPipeline(releaseID, appName, commit string, version int) {
	log.Printf("[build] starting local pipeline app=%s commit=%s release=%s", appName, commit, releaseID)

	buildOut, err := bm.runBuilder(appName, commit)
	if err != nil {
		log.Printf("[build] builder failed app=%s: %v", appName, err)
		bm.updateRelease(releaseID, "failed", buildOut+"\nBuild error: "+err.Error())
		return
	}

	imageTag := fmt.Sprintf("%s/%s:%s", bm.config.RegistryURL, appName, commit)
	deployOut, err := bm.runDeployer(appName, imageTag)
	combined := buildOut + "\n" + deployOut
	if err != nil {
		log.Printf("[build] deployer failed app=%s: %v", appName, err)
		bm.updateRelease(releaseID, "failed", combined+"\nDeploy error: "+err.Error())
		return
	}

	log.Printf("[build] local pipeline succeeded app=%s v%d", appName, version)
	bm.updateRelease(releaseID, "succeeded", combined)
}

func (bm *BuildManager) runBuilder(appName, commit string) (string, error) {
	repoPath := filepath.Join(bm.config.ReposDir, appName+".git")
	cmd := exec.Command("bash", bm.config.BuilderScript, appName, commit, repoPath)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (bm *BuildManager) runDeployer(appName, image string) (string, error) {
	cmd := exec.Command("bash", bm.config.DeployerScript, appName, image, "1")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (bm *BuildManager) updateRelease(id, status, output string) {
	if len(output) > 64000 {
		output = output[:64000]
	}
	_, err := bm.db.Exec(
		fmt.Sprintf("UPDATE releases SET status = %s, build_output = %s WHERE id = %s",
			bm.ph(1), bm.ph(2), bm.ph(3)),
		status, output, id,
	)
	if err != nil {
		log.Printf("[build] failed to update release %s: %v", id, err)
	}
}

func (bm *BuildManager) GetBuildLogs(releaseID string) (string, string, error) {
	var status, output string
	err := bm.db.QueryRow(
		fmt.Sprintf("SELECT status, COALESCE(build_output, '') FROM releases WHERE id = %s", bm.ph(1)),
		releaseID,
	).Scan(&status, &output)
	return status, output, err
}
