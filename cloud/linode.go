package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/linode/linodego"
	"golang.org/x/oauth2"

	"redwood.dev/errors"
)

var (
	Err404 = errors.New("not found")
)

type LinodeClient struct {
	client linodego.Client
}

var _ Client = (*LinodeClient)(nil)

func NewLinodeClient(apiKey string) *LinodeClient {
	return &LinodeClient{
		client: linodego.NewClient(&http.Client{
			Transport: &oauth2.Transport{
				Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey}),
			},
		}),
	}
}

func (c *LinodeClient) CreateStack(ctx context.Context, opts CreateStackOptions) (err error) {
	instance, err := c.ensureInstance(ctx, opts)
	if err != nil {
		return err
	}
	err = c.ensureDomain(ctx, instance, opts)
	if err != nil {
		return err
	}
	return nil
}

const stackScriptTemplate = `#!/bin/sh

apt update
apt -y install \
    apt-transport-https \
    ca-certificates \
    curl \
    software-properties-common
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
sudo add-apt-repository \
   "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
   $(lsb_release -cs) \
   stable"

apt update
apt -y install docker-ce
systemctl enable docker
service docker start

mkdir -p /root/config
cat <<EOF > /root/config/.redwoodrc
Node:
    SubscribedStateURIs:
      - %v
    DevMode: true
P2PTransport:
    Enabled: true
    ListenAddr: 0.0.0.0
    ListenPort: 21231
HTTPTransport:
    Enabled: true
    ListenHost: :8080
HTTPRPC:
    Enabled: true
    ListenHost: :8081
    Whitelist:
        Enabled: true
        PermittedAddrs:
          - %v
EOF

docker run -v /root/config:/config -p 8080:8080 -p 8081:8081 -p 21231:21231 brynbellomy/redwood-chat
`

func (c *LinodeClient) ensureInstance(ctx context.Context, opts CreateStackOptions) (instance *linodego.Instance, err error) {
	defer func() { err = errors.WithStack(err) }()

	instance, err = c.findInstance(ctx, opts.InstanceLabel)
	if errors.Cause(err) == Err404 {
		script, err := c.client.CreateStackscript(ctx, linodego.StackscriptCreateOptions{
			Description: "redwood-chat--" + opts.DomainName,
			Images:      []string{opts.InstanceImage},
			Label:       opts.InstanceLabel,
			Script:      fmt.Sprintf(stackScriptTemplate, opts.FirstStateURI, opts.AdminAddress),
		})
		if err != nil {
			return nil, errors.Wrap(err, "while creating stack script")
		}

		swapSize := 1024
		booted := true
		instance, err = c.client.CreateInstance(ctx, linodego.InstanceCreateOptions{
			Region:         opts.InstanceRegion,
			Type:           opts.InstanceType,
			Label:          opts.InstanceLabel,
			RootPass:       opts.InstancePassword,
			Image:          opts.InstanceImage,
			BackupsEnabled: true,
			Tags:           []string{"redwood", "redwood-" + opts.DomainName},
			SwapSize:       &swapSize,
			Booted:         &booted,
			// AuthorizedKeys  []string          `json:"authorized_keys,omitempty"`
			// AuthorizedUsers []string          `json:"authorized_users,omitempty"`
			StackScriptID: script.ID,
			// StackScriptData: map[string]string{},
			// BackupID        int               `json:"backup_id,omitempty"`
			// PrivateIP       bool              `json:"private_ip,omitempty"`
		})
		return instance, errors.Wrap(err, "while creating instance")
	} else if err != nil {
		return nil, errors.Wrap(err, "while trying to fetch instance")
	}
	fmt.Println("Instance:", prettyJSON(instance))
	return
}

func (c *LinodeClient) ensureDomain(ctx context.Context, instance *linodego.Instance, opts CreateStackOptions) (err error) {
	defer func() { err = errors.WithStack(err) }()

	var rootDomainName string
	var subdomain string
	parts := strings.Split(opts.DomainName, ".")
	if len(parts) > 2 {
		rootDomainName = strings.Join(parts[len(parts)-2:], ".")
		subdomain = strings.Join(parts[:len(parts)-2], ".")
	} else {
		rootDomainName = opts.DomainName
	}

	//
	// Domain
	//

	domain, err := c.findDomain(ctx, rootDomainName)
	if errors.Cause(err) == Err404 {
		d, err := c.client.CreateDomain(ctx, linodego.DomainCreateOptions{
			Domain:      rootDomainName,
			Type:        linodego.DomainTypeMaster,
			Status:      linodego.DomainStatusActive,
			Description: fmt.Sprintf("Redwood (%v)", rootDomainName),
			SOAEmail:    opts.DomainEmail,
			RetrySec:    0,          // Valid values are 300, 3600, 7200, 14400, 28800, 57600, 86400, 172800, 345600, 604800, 1209600, and 2419200
			MasterIPs:   []string{}, // The IP addresses representing the master DNS for this Domain.
			ExpireSec:   0,          // The amount of time in seconds that may pass before this Domain is no longer authoritative. Valid values are 300, 3600, 7200, 14400, 28800, 57600, 86400, 172800, 345600, 604800, 1209600, and 2419200 - any other value will be rounded to the nearest valid value.
			RefreshSec:  0,          // The amount of time in seconds before this Domain should be refreshed. Valid values are 300, 3600, 7200, 14400, 28800, 57600, 86400, 172800, 345600, 604800, 1209600, and 2419200 - any other value will be rounded to the nearest valid value.
			TTLSec:      0,          // "Time to Live" - the amount of time in seconds that this Domain's records may be cached by resolvers or other domain servers. Valid values are 300, 3600, 7200, 14400, 28800, 57600, 86400, 172800, 345600, 604800, 1209600, and 2419200 - any other value will be rounded to the nearest valid value.
			Tags:        []string{"redwood", "redwood-" + rootDomainName},
		})
		if err != nil {
			return errors.Wrapf(err, "while creating domain (%v)", rootDomainName)
		}
		domain = *d
	} else if err != nil {
		return errors.Wrap(err, "while trying to find existing domain")
	}
	fmt.Println("Domain:", prettyJSON(domain))

	//
	// Domain records
	//

	// First, delete any that aren't pointing at one of the IP addresses of the instance
	records, err := c.client.ListDomainRecords(ctx, domain.ID, nil)
	if err != nil {
		return err
	}
	for _, r := range records {
		switch r.Type {
		default:
			continue

		case linodego.RecordTypeA:
			if !containsIP(instance.IPv4, r.Target) {
				err = c.client.DeleteDomainRecord(ctx, domain.ID, r.ID)
				if err != nil {
					return err
				}
			}

		case linodego.RecordTypeAAAA:
			if r.Target != fixIPv6(instance.IPv6) {
				err = c.client.DeleteDomainRecord(ctx, domain.ID, r.ID)
				if err != nil {
					return err
				}
			}
		}
	}

	// Now ensure that we have every type of record we need
	for _, ipv4 := range instance.IPv4 {
		record, err := c.findDomainRecord(ctx, domain.ID, linodego.RecordTypeA, ipv4.String(), "")
		if errors.Cause(err) == Err404 {
			_, err := c.client.CreateDomainRecord(ctx, domain.ID, linodego.DomainRecordCreateOptions{
				Type:   linodego.RecordTypeA,
				Target: ipv4.String(),
				TTLSec: 0,
			})
			if err != nil {
				return errors.Wrap(err, "while creating domain record")
			}
		} else if err != nil {
			return err
		}
		fmt.Println("Record:", prettyJSON(record))
	}

	record, err := c.findDomainRecord(ctx, domain.ID, linodego.RecordTypeAAAA, fixIPv6(instance.IPv6), "")
	if errors.Cause(err) == Err404 {
		_, err = c.client.CreateDomainRecord(ctx, domain.ID, linodego.DomainRecordCreateOptions{
			Type:   linodego.RecordTypeAAAA,
			Target: fixIPv6(instance.IPv6),
			TTLSec: 0,
		})
		if err != nil {
			return err
		}
	}
	fmt.Println("Record:", prettyJSON(record))

	// Handle subdomains
	if subdomain != "" {
		for _, ipv4 := range instance.IPv4 {
			record, err := c.findDomainRecord(ctx, domain.ID, linodego.RecordTypeA, ipv4.String(), subdomain)
			if errors.Cause(err) == Err404 {
				_, err := c.client.CreateDomainRecord(ctx, domain.ID, linodego.DomainRecordCreateOptions{
					Type:   linodego.RecordTypeA,
					Target: ipv4.String(),
					Name:   subdomain,
					TTLSec: 0,
				})
				if err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			fmt.Println("Record:", prettyJSON(record))
		}

		record, err := c.findDomainRecord(ctx, domain.ID, linodego.RecordTypeAAAA, fixIPv6(instance.IPv6), subdomain)
		if errors.Cause(err) == Err404 {
			_, err = c.client.CreateDomainRecord(ctx, domain.ID, linodego.DomainRecordCreateOptions{
				Type:   linodego.RecordTypeAAAA,
				Target: fixIPv6(instance.IPv6),
				Name:   subdomain,
				TTLSec: 0,
			})
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
		fmt.Println("Record:", prettyJSON(record))
	}

	return nil
}

func (c *LinodeClient) findInstance(ctx context.Context, instanceLabel string) (instance *linodego.Instance, err error) {
	instances, err := c.client.ListInstances(ctx, nil)
	if err != nil {
		return
	}
	for _, i := range instances {
		if i.Label == instanceLabel {
			return &i, nil
		}
	}
	return instance, Err404
}

func (c *LinodeClient) findDomain(ctx context.Context, rootDomainName string) (domain linodego.Domain, err error) {
	domains, err := c.client.ListDomains(ctx, nil)
	if err != nil {
		return
	}
	for _, d := range domains {
		if d.Domain == rootDomainName {
			return d, nil
		}
	}
	return domain, Err404
}

func (c *LinodeClient) findDomainRecord(
	ctx context.Context,
	domainID int,
	recordType linodego.DomainRecordType,
	target string,
	name string,
) (record linodego.DomainRecord, err error) {
	records, err := c.client.ListDomainRecords(ctx, domainID, nil)
	if err != nil {
		return record, err
	}
	for _, r := range records {
		if r.Type == recordType && r.Target == target && r.Name == name {
			return r, nil
		}
	}
	return record, Err404
}

func (c *LinodeClient) SSHKeys(ctx context.Context) ([]SSHKey, error) {
	keys, err := c.client.ListSSHKeys(ctx, nil)
	if err != nil {
		return nil, err
	}
	var sshKeys []SSHKey
	for _, key := range keys {
		sshKeys = append(sshKeys, SSHKey{Label: key.Label})
	}
	return sshKeys, nil
}

func (c *LinodeClient) Regions(ctx context.Context) ([]Region, error) {
	regions, err := c.client.ListRegions(ctx, nil)
	if err != nil {
		return nil, err
	}
	var rs []Region
	for _, r := range regions {
		rs = append(rs, Region{ID: r.ID, Geo: r.Country})
	}
	return rs, nil
}

func (c *LinodeClient) InstanceTypes(ctx context.Context) ([]InstanceType, error) {
	types, err := c.client.ListTypes(ctx, nil)
	if err != nil {
		return nil, err
	}
	var is []InstanceType
	for _, i := range types {
		is = append(is, InstanceType{
			ID:           i.ID,
			DiskSpaceMB:  i.Disk,
			MemoryMB:     i.Memory,
			NumCPUs:      i.VCPUs,
			HourlyPrice:  i.Price.Hourly,
			MonthlyPrice: i.Price.Monthly,
		})
	}
	return is, nil
}

func (c *LinodeClient) Images(ctx context.Context) ([]Image, error) {
	images, err := c.client.ListImages(ctx, nil)
	if err != nil {
		return nil, err
	}
	var is []Image
	for _, i := range images {
		is = append(is, Image{
			ID:          i.ID,
			Label:       i.Label,
			Description: i.Description,
			Type:        i.Type,
			Vendor:      i.Vendor,
			Size:        i.Size,
		})
	}
	return is, nil
}

func fixIPv6(ip string) string {
	return strings.Split(ip, "/")[0]
}

func containsIP(ips []*net.IP, needle string) bool {
	for _, ip := range ips {
		if ip.String() == needle {
			return true
		}
	}
	return false
}

func prettyJSON(x interface{}) string {
	bs, _ := json.MarshalIndent(x, "", "    ")
	return string(bs)
}
