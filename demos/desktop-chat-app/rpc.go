package main

import (
	"context"
	"net/http"

	"github.com/brynbellomy/redwood"
	"github.com/brynbellomy/redwood/cloud"
)

type HTTPRPCServer struct {
	*redwood.HTTPRPCServer
}

type (
	CreateStackOptionsArgs struct {
		Provider string `json:"provider"`
		APIKey   string `json:"apiKey"`
	}
	CreateStackOptionsResponse struct {
		SSHKeys       []cloud.SSHKey       `json:"sshKeys"`
		Regions       []cloud.Region       `json:"regions"`
		InstanceTypes []cloud.InstanceType `json:"instanceTypes"`
		Images        []cloud.Image        `json:"images"`
	}
)

func (s *HTTPRPCServer) CreateStackOptions(r *http.Request, args *CreateStackOptionsArgs, resp *CreateStackOptionsResponse) error {
	c, err := cloud.NewClient(args.Provider, args.APIKey)
	if err != nil {
		return err
	}

	ctx := context.Background()

	resp.SSHKeys, err = c.SSHKeys(ctx)
	if err != nil {
		return err
	}
	resp.Regions, err = c.Regions(ctx)
	if err != nil {
		return err
	}
	resp.InstanceTypes, err = c.InstanceTypes(ctx)
	if err != nil {
		return err
	}
	resp.Images, err = c.Images(ctx)
	if err != nil {
		return err
	}
	return nil
}

type (
	CreateStackArgs struct {
		Provider         string `json:"provider"`
		APIKey           string `json:"apiKey"`
		DomainName       string `json:"domainName"`
		DomainEmail      string `json:"domainEmail"`
		InstanceLabel    string `json:"instanceLabel"`
		InstanceRegion   string `json:"instanceRegion"`
		InstancePassword string `json:"instancePassword"`
		InstanceType     string `json:"instanceType"`
		InstanceImage    string `json:"instanceImage"`
		InstanceSSHKey   string `json:"instanceSSHKey"`
	}
	CreateStackResponse struct{}
)

func (s *HTTPRPCServer) CreateStack(r *http.Request, args *CreateStackArgs, resp *CreateStackResponse) error {
	c, err := cloud.NewClient(args.Provider, args.APIKey)
	if err != nil {
		return err
	}
	ctx := context.Background()
	return c.CreateStack(ctx, cloud.CreateStackOptions{
		DomainName:       args.DomainName,
		DomainEmail:      args.DomainEmail,
		InstanceLabel:    args.InstanceLabel,
		InstanceRegion:   args.InstanceRegion,
		InstancePassword: args.InstancePassword,
		InstanceType:     args.InstanceType,
		InstanceImage:    args.InstanceImage,
		InstanceSSHKey:   args.InstanceSSHKey,
	})
}
