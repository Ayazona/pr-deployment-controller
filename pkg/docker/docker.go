package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

// Docker defines the interface used to talk with Docker.
type Docker interface {
	// BuildImage uploads the build context and requests an image build
	BuildImage(ctx context.Context, buildContext io.Reader, image, dockerfile string) error
	// PushImage pushes an image to a remote repository
	PushImage(ctx context.Context, image string) error
	// RegistryName generates a image name
	ImageName(owner, repository, ref string) string
}

// Config stores the config for the docker controller
type Config struct {
	Host       string
	APIVersion string

	CertFile   string
	KeyFile    string
	CACertFile string

	Registry             string
	RegistryUsername     string
	RegistryPassword     string
	RegistryPasswordFile string
}

type baseDocker struct {
	logger *logrus.Entry
	c      *client.Client

	registry string
	username string
	password string
}

// New creates a new Docker controller
func New(logger *logrus.Entry, config *Config) (Docker, error) {
	// Setup the http client
	httpClient, err := newHTTPClient(config.CertFile, config.KeyFile, config.CACertFile)
	if err != nil {
		return nil, err
	}

	// Setup the docker client
	c, err := client.NewClient(
		config.Host,
		config.APIVersion,
		httpClient,
		nil,
	)
	if err != nil {
		return nil, err
	}

	var registryPassword string

	if config.RegistryPassword != "" {
		registryPassword = config.RegistryPassword
	} else if config.RegistryPasswordFile != "" {
		// Open password file
		f, err := os.Open(config.RegistryPasswordFile)
		if err != nil {
			return nil, err
		}

		// Read file
		content, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}

		registryPassword = string(content)
	}

	return &baseDocker{
		logger:   logger,
		c:        c,
		registry: config.Registry,
		username: config.RegistryUsername,
		password: registryPassword,
	}, nil
}

// BuildImage sends a build context to the docker daemon and instructs the daemon to build an image
func (d *baseDocker) BuildImage(
	ctx context.Context,
	buildContext io.Reader,
	image,
	dockerfile string,
) error {
	d.logger.WithField("image", image).Infof("building image")

	resp, err := d.c.ImageBuild(ctx, buildContext, types.ImageBuildOptions{
		Tags:       []string{image},
		Dockerfile: dockerfile,
	})
	if err != nil {
		return err
	}

	return checkResponse(resp.Body)
}

// PushImage instructs the docker daemon to push an image to an external registry
// nolint: gocyclo
func (d *baseDocker) PushImage(ctx context.Context, image string) error {
	options := types.ImagePushOptions{}

	logger := d.logger.WithField("image", image)

	// Create base64 encoded auth credentials
	if d.username != "" && d.password != "" {
		authConfig := types.AuthConfig{
			Username: d.username,
			Password: d.password,
		}

		encodedJSON, err := json.Marshal(authConfig)
		if err != nil {
			return err
		}

		options.RegistryAuth = base64.URLEncoding.EncodeToString(encodedJSON)
	}

	if options.RegistryAuth != "" {
		logger.Info("pushing image with credentials")
	} else {
		logger.Infof("pushing image")
	}

	resp, err := d.c.ImagePush(ctx, image, options)
	if err != nil {
		return err
	}

	return checkResponse(resp)
}

// ImageName returns the docker image name based on repository, owner and ref
func (d *baseDocker) ImageName(owner, repository, ref string) string {
	if d.registry != "" {
		return fmt.Sprintf("%s/%s/%s:%s", d.registry, owner, repository, ref)
	}

	return fmt.Sprintf("%s/%s:%s", owner, repository, ref)
}
