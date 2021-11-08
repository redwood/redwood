package cloud

import (
	"context"

	"redwood.dev/errors"
)

type Client interface {
	CreateStack(ctx context.Context, opts CreateStackOptions) error
	SSHKeys(ctx context.Context) ([]SSHKey, error)
	Regions(ctx context.Context) ([]Region, error)
	InstanceTypes(ctx context.Context) ([]InstanceType, error)
	Images(ctx context.Context) ([]Image, error)
}

func NewClient(provider, apiKey string) (Client, error) {
	switch provider {
	case "linode":
		return NewLinodeClient(apiKey), nil
	default:
		return nil, errors.New("unknown provider")
	}
}

type CreateStackOptions struct {
	FirstStateURI string `json:"firstStateURI"`
	AdminAddress  string `json:"adminAddress"`

	DomainName       string `json:"domainName"`
	DomainEmail      string `json:"domainEmail"`
	InstanceLabel    string `json:"instanceLabel"`
	InstanceRegion   string `json:"instanceRegion"`
	InstancePassword string `json:"instancePassword"`
	InstanceType     string `json:"instanceType"`
	InstanceImage    string `json:"instanceImage"`
	InstanceSSHKey   string `json:"instanceSSHKey"`
}

type SSHKey struct {
	Label string `json:"label"`
}

type Region struct {
	ID  string `json:"id"`
	Geo string `json:"geo"`
}

type InstanceType struct {
	ID           string  `json:"id"`
	DiskSpaceMB  int     `json:"diskSpaceMB"`
	MemoryMB     int     `json:"memoryMB"`
	NumCPUs      int     `json:"numCPUs"`
	HourlyPrice  float32 `json:"hourlyPrice"`
	MonthlyPrice float32 `json:"monthlyPrice"`
}

type Image struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Vendor      string `json:"vendor"`
	Size        int    `json:"size"`
}
