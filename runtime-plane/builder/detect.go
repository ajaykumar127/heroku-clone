package main

import "os"

// AppType identifies the type of application detected in a build directory.
type AppType string

const (
	AppTypeNode       AppType = "nodejs"
	AppTypePython     AppType = "python"
	AppTypeRuby       AppType = "ruby"
	AppTypeGo         AppType = "go"
	AppTypeJava       AppType = "java"
	AppTypeDockerfile AppType = "dockerfile"
	AppTypeUnknown    AppType = "unknown"
)

// Detect inspects files in buildDir to determine the application type.
// Detection is performed in priority order: Dockerfile first, then language
// indicators, falling back to AppTypeUnknown when nothing matches.
func Detect(buildDir string) AppType {
	exists := func(name string) bool {
		_, err := os.Stat(buildDir + "/" + name)
		return err == nil
	}

	switch {
	case exists("Dockerfile"):
		return AppTypeDockerfile
	case exists("package.json"):
		return AppTypeNode
	case exists("requirements.txt") || exists("Pipfile") || exists("pyproject.toml"):
		return AppTypePython
	case exists("Gemfile"):
		return AppTypeRuby
	case exists("go.mod"):
		return AppTypeGo
	case exists("pom.xml") || exists("build.gradle"):
		return AppTypeJava
	default:
		return AppTypeUnknown
	}
}
