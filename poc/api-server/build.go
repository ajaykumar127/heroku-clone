package main

import (
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
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
// Postgres uses $1,$2,...; SQLite uses ?.
func (bm *BuildManager) ph(n int) string {
	if bm.postgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// TriggerBuild runs the build+deploy pipeline asynchronously.
func (bm *BuildManager) TriggerBuild(appName, commit string) (string, error) {
	var appID string
	err := bm.db.QueryRow(
		fmt.Sprintf("SELECT id FROM apps WHERE name = %s", bm.ph(1)), appName,
	).Scan(&appID)
	if err != nil {
		return "", fmt.Errorf("app %q not found: %w", appName, err)
	}

	releaseID, version, err := bm.createRelease(appID, commit)
	if err != nil {
		return "", fmt.Errorf("failed to create release: %w", err)
	}

	go bm.runPipeline(releaseID, appName, commit, version)
	return releaseID, nil
}

func (bm *BuildManager) createRelease(appID, commit string) (string, int, error) {
	var version int
	err := bm.db.QueryRow(
		fmt.Sprintf("SELECT COALESCE(MAX(version), 0) + 1 FROM releases WHERE app_id = %s", bm.ph(1)),
		appID,
	).Scan(&version)
	if err != nil {
		return "", 0, err
	}

	id := uuid.New().String()
	now := time.Now()
	_, err = bm.db.Exec(
		fmt.Sprintf(
			`INSERT INTO releases (id, app_id, version, commit, status, build_output, created_at)
			 VALUES (%s, %s, %s, %s, 'building', '', %s)`,
			bm.ph(1), bm.ph(2), bm.ph(3), bm.ph(4), bm.ph(5),
		),
		id, appID, version, commit, now,
	)
	return id, version, err
}

func (bm *BuildManager) runPipeline(releaseID, appName, commit string, version int) {
	log.Printf("[build] starting pipeline app=%s commit=%s release=%s", appName, commit, releaseID)

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

	log.Printf("[build] pipeline succeeded app=%s v%d", appName, version)
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

