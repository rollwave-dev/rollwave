package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Options defines the parameters for the build process.
type Options struct {
	ImageName  string // e.g. ttl.sh/my-app
	ContextDir string // e.g. .
	Dockerfile string // e.g. Dockerfile
	Stdout     io.Writer
	Stderr     io.Writer
}

// Login checks for ROLLWAVE_REGISTRY_USER and ROLLWAVE_REGISTRY_PASSWORD.
// If present, it extracts the registry server from the imageName and performs 'docker login'.
func Login(ctx context.Context, imageName string, stdout, stderr io.Writer) error {
	user := os.Getenv("ROLLWAVE_REGISTRY_USER")
	pass := os.Getenv("ROLLWAVE_REGISTRY_PASSWORD")

	if user == "" || pass == "" {
		return nil
	}

	registry := extractRegistry(imageName)

	targetDesc := registry
	if targetDesc == "" {
		targetDesc = "Docker Hub"
	}

	fmt.Fprintf(stdout, "🔑 Authenticating to %s as user '%s'...\n", targetDesc, user)

	args := []string{"login", "-u", user, "--password-stdin"}
	if registry != "" {
		args = append(args, registry)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(pass)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker login failed: %w", err)
	}

	return nil
}

// Run executes the docker build, tag, and push commands.
// It returns the full image name including the generated tag.
func Run(ctx context.Context, opt Options) (string, error) {
	if opt.Stdout == nil {
		opt.Stdout = os.Stdout
	}
	if opt.Stderr == nil {
		opt.Stderr = os.Stderr
	}

	// 1. Generate Tag (Git hash or Timestamp)
	tag := getGitTag()
	fullImage := fmt.Sprintf("%s:%s", opt.ImageName, tag)
	latestImage := fmt.Sprintf("%s:latest", opt.ImageName)

	fmt.Fprintf(opt.Stdout, "📦 Building image: %s\n", fullImage)

	// 2. Docker Build
	buildCmd := exec.CommandContext(ctx, "docker", "build",
		"-t", fullImage,
		"-t", latestImage,
		"-f", opt.Dockerfile,
		opt.ContextDir,
	)
	buildCmd.Stdout = opt.Stdout
	buildCmd.Stderr = opt.Stderr

	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("docker build failed: %w", err)
	}

	// 3. Docker Push (Versioned)
	fmt.Fprintf(opt.Stdout, "⬆️  Pushing image %s ...\n", fullImage)
	if err := pushImage(ctx, fullImage, opt.Stdout, opt.Stderr); err != nil {
		return "", err
	}

	// 4. Docker Push (Latest) - allows deploy without build
	fmt.Fprintf(opt.Stdout, "⬆️  Pushing image %s ...\n", latestImage)
	if err := pushImage(ctx, latestImage, opt.Stdout, opt.Stderr); err != nil {
		return "", err
	}

	return fullImage, nil
}

func pushImage(ctx context.Context, image string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "docker", "push", image)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// getGitTag returns the short git hash or a timestamp if git is unavailable.
func getGitTag() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return fmt.Sprintf("v%d", time.Now().Unix())
}

func extractRegistry(image string) string {
	parts := strings.Split(image, "/")
	if len(parts) > 0 {
		domain := parts[0]
		if strings.Contains(domain, ".") || strings.Contains(domain, ":") || domain == "localhost" {
			return domain
		}
	}
	return ""
}
