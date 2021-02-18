package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"redwood.dev"
	"redwood.dev/cloud"
)

type HTTPRPCServer struct {
	*redwood.HTTPRPCServer
	signingKeypair *redwood.SigningKeypair
}

type (
	CreateCloudStackOptionsArgs struct {
		Provider string `json:"provider"`
		APIKey   string `json:"apiKey"`
	}
	CreateCloudStackOptionsResponse struct {
		SSHKeys       []cloud.SSHKey       `json:"sshKeys"`
		Regions       []cloud.Region       `json:"regions"`
		InstanceTypes []cloud.InstanceType `json:"instanceTypes"`
		Images        []cloud.Image        `json:"images"`
	}
)

func (s *HTTPRPCServer) CreateCloudStackOptions(r *http.Request, args *CreateCloudStackOptionsArgs, resp *CreateCloudStackOptionsResponse) error {
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
	CreateCloudStackArgs struct {
		FirstStateURI    string `json:"firstStateURI"`
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
	CreateCloudStackResponse struct{}
)

func (s *HTTPRPCServer) CreateCloudStack(r *http.Request, args *CreateCloudStackArgs, resp *CreateCloudStackResponse) error {
	bs, _ := json.MarshalIndent(args, "", "    ")
	fmt.Println("create stack", string(bs))
	c, err := cloud.NewClient(args.Provider, args.APIKey)
	if err != nil {
		return err
	}
	ctx := context.Background()
	return c.CreateStack(ctx, cloud.CreateStackOptions{
		FirstStateURI:    args.FirstStateURI,
		AdminAddress:     s.signingKeypair.Address().Hex(),
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

type (
	CloudNodeSubscribeArgs struct {
		RemoteRPCHost string `json:"remoteRPCHost"`
		StateURI      string `json:"stateURI"`
	}
	CloudNodeSubscribeResponse struct{}
)

func (s *HTTPRPCServer) CloudNodeSubscribe(r *http.Request, args *CloudNodeSubscribeArgs, resp *CloudNodeSubscribeResponse) error {
	client := redwood.NewHTTPRPCClient(args.RemoteRPCHost)
	defer client.Close()

	err := client.Authorize(s.signingKeypair)
	if err != nil {
		return err
	}
	return client.Subscribe(redwood.RPCSubscribeArgs{StateURI: args.StateURI, Txs: true})
}
