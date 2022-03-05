package azure

import (
	"context"
	"os"
	"strings"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"

	publicAzureDNS "github.com/Azure/azure-sdk-for-go/profiles/latest/dns/mgmt/dns"
	privateAzureDNS "github.com/Azure/azure-sdk-for-go/profiles/latest/privatedns/mgmt/privatedns"
	"github.com/Azure/go-autorest/autorest/azure/auth"
)

var log = clog.NewWithPlugin("azure")

func init() { plugin.Register("azure", setup) }

func setup(c *caddy.Controller) error {
	keys, accessMap, fall, err := parse(c)
	if err != nil {
		return plugin.Error("azure", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return plugin.Error("azure", err)
	}

	publicDNSClient := publicAzureDNS.NewRecordSetsClient(os.Getenv(auth.SubscriptionID))
	publicDNSClient.Authorizer = authorizer

	privateDNSClient := privateAzureDNS.NewRecordSetsClient(os.Getenv(auth.SubscriptionID))
	privateDNSClient.Authorizer = authorizer

	h, err := New(ctx, publicDNSClient, privateDNSClient, keys, accessMap)
	if err != nil {
		return plugin.Error("azure", err)
	}
	h.Fall = fall
	if err := h.Run(ctx); err != nil {
		return plugin.Error("azure", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		h.Next = next
		return h
	})
	c.OnShutdown(func() error { cancel(); return nil })
	return nil
}

func parse(c *caddy.Controller) (map[string][]string, map[string]string, fall.F, error) {
	resourceGroupMapping := map[string][]string{}
	accessMap := map[string]string{}
	resourceGroupSet := map[string]struct{}{}

	var fall fall.F
	var access string
	var resourceGroup string
	var zoneName string

	for c.Next() {
		args := c.RemainingArgs()

		for i := 0; i < len(args); i++ {
			parts := strings.SplitN(args[i], ":", 2)
			if len(parts) != 2 {
				return resourceGroupMapping, accessMap, fall, c.Errf("invalid resource group/zone: %q", args[i])
			}
			resourceGroup, zoneName = parts[0], parts[1]
			if resourceGroup == "" || zoneName == "" {
				return resourceGroupMapping, accessMap, fall, c.Errf("invalid resource group/zone: %q", args[i])
			}
			if _, ok := resourceGroupSet[resourceGroup+zoneName]; ok {
				return resourceGroupMapping, accessMap, fall, c.Errf("conflicting zone: %q", args[i])
			}

			resourceGroupSet[resourceGroup+zoneName] = struct{}{}
			accessMap[resourceGroup+zoneName] = "public"
			resourceGroupMapping[resourceGroup] = append(resourceGroupMapping[resourceGroup], zoneName)
		}

		for c.NextBlock() {
			switch c.Val() {
			case "subscription":
				if !c.NextArg() {
					return resourceGroupMapping, accessMap, fall, c.ArgErr()
				}
				os.Setenv(auth.SubscriptionID, c.Val())
			case "tenant":
				if !c.NextArg() {
					return resourceGroupMapping, accessMap, fall, c.ArgErr()
				}
				os.Setenv(auth.TenantID, c.Val())
			case "client":
				if !c.NextArg() {
					return resourceGroupMapping, accessMap, fall, c.ArgErr()
				}
				os.Setenv(auth.ClientID, c.Val())
			case "secret":
				if !c.NextArg() {
					return resourceGroupMapping, accessMap, fall, c.ArgErr()
				}
				os.Setenv(auth.ClientSecret, c.Val())
				log.Warningf("Save secret in Corefile has been deprecated, please use environmental variable 'AZURE_CLIENT_SECRET' instead")
			case "environment":
				if !c.NextArg() {
					return resourceGroupMapping, accessMap, fall, c.ArgErr()
				}
				os.Setenv(auth.EnvironmentName, c.Val())
			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())
			case "access":
				if !c.NextArg() {
					return resourceGroupMapping, accessMap, fall, c.ArgErr()
				}
				access = c.Val()
				if access != "public" && access != "private" {
					return resourceGroupMapping, accessMap, fall, c.Errf("invalid access value: can be public/private, found: %s", access)
				}
				accessMap[resourceGroup+zoneName] = access
			default:
				return resourceGroupMapping, accessMap, fall, c.Errf("unknown property: %q", c.Val())
			}
		}
	}

	return resourceGroupMapping, accessMap, fall, nil
}
