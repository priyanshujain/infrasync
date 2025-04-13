package cloudsql

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) ListInstances(projectID string) ([]*sqladmin.DatabaseInstance, error) {
	cmd := exec.Command("gcloud", "sql", "instances", "list", fmt.Sprintf("--project=%s", projectID), "--format=json")

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, errors.New("failed to execute gcloud command: " + err.Error())
	}

	var instances []*sqladmin.DatabaseInstance
	if err := json.Unmarshal(out.Bytes(), &instances); err != nil {
		return nil, errors.New("failed to parse gcloud output: " + err.Error())
	}

	return instances, nil

}
