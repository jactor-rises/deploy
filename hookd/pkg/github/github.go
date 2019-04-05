package github

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	gh "github.com/google/go-github/v23/github"
	types "github.com/navikt/deployment/common/pkg/deployment"
)

const maxDescriptionLength = 140

func SplitFullname(fullName string) (string, string, error) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("repository name %s is not in the format OWNER/NAME", fullName)
	}
	return parts[0], parts[1], nil
}

func InstallationClient(appId, installId int, keyFile string) (*gh.Client, error) {
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installId, keyFile)
	if err != nil {
		return nil, err
	}
	return gh.NewClient(&http.Client{Transport: itr}), nil
}

func CreateDeploymentStatus(client *gh.Client, m *types.DeploymentStatus, baseurl string) (*gh.DeploymentStatus, *gh.Response, error) {
	if client == nil {
		return nil, nil, fmt.Errorf("no Github client supplied")
	}

	deployment := m.GetDeployment()
	if deployment == nil {
		return nil, nil, fmt.Errorf("empty deployment")
	}

	repo := deployment.GetRepository()
	if repo == nil {
		return nil, nil, fmt.Errorf("empty repository")
	}

	state := m.GetState().String()
	description := m.GetDescription()
	if len(description) > maxDescriptionLength {
		description = description[:maxDescriptionLength]
	}

	unixTime := time.Now().Unix()
	url := path.Join(baseurl, "logs") + fmt.Sprintf("delivery_id=%s&ts=%d", m.GetDeliveryID(), unixTime)

	return client.Repositories.CreateDeploymentStatus(
		context.Background(),
		repo.GetOwner(),
		repo.GetName(),
		deployment.GetDeploymentID(),
		&gh.DeploymentStatusRequest{
			State:       &state,
			Description: &description,
			LogURL:      &url,
		},
	)
}
