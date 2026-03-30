package provider

import (
	"context"
	"fmt"

	"github.com/yukariin/external-dns-openwrt-webhook/pkg/logger"
	"github.com/yukariin/external-dns-openwrt-webhook/pkg/openwrt"
	"go.uber.org/zap"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

const defaultTTL = 300

type Provider struct {
	provider.BaseProvider

	openwrt openwrt.OpenWRT
}

func New(cfg *Config) (*Provider, error) {
	opwrt, err := openwrt.New(cfg.OpenWRT)
	if err != nil {
		return nil, err
	}

	return &Provider{
		openwrt: opwrt,
	}, nil
}

func (p *Provider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	logger.Log.Debug("apply changes", zap.Any("changes", changes))

	// Phase 1: Delete stale records (UpdateOld + Delete)
	toDelete := append(
		endpoints2DNSRecords(changes.UpdateOld),
		endpoints2DNSRecords(changes.Delete)...,
	)
	if len(toDelete) > 0 {
		if err := p.openwrt.DeleteDNSRecords(ctx, toDelete); err != nil {
			return fmt.Errorf("delete phase failed: %w", err)
		}
	}

	// Phase 2: Create new records (Create + UpdateNew)
	toCreate := append(
		endpoints2DNSRecords(changes.Create),
		endpoints2DNSRecords(changes.UpdateNew)...,
	)
	if len(toCreate) > 0 {
		if err := p.openwrt.SetDNSRecords(ctx, toCreate); err != nil {
			return fmt.Errorf("create phase failed: %w", err)
		}
	}

	return nil
}

func (p *Provider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	records, err := p.openwrt.GetDNSRecords(ctx)
	if err != nil {
		return nil, err
	}

	return dnsRecords2Endpoints(records), nil
}

func dnsRecords2Endpoints(dnsRecords map[string]openwrt.DNSRecord) []*endpoint.Endpoint {
	var endpoints []*endpoint.Endpoint

	for uciSection, dnsRecord := range dnsRecords {
		var ep endpoint.Endpoint

		switch dnsRecord.Type {
		case "A":
			ep.RecordType = endpoint.RecordTypeA
			ep.DNSName = dnsRecord.Name
			ep.Targets = endpoint.Targets{dnsRecord.IP}
		case "CNAME":
			ep.RecordType = endpoint.RecordTypeCNAME
			ep.DNSName = dnsRecord.CName
			ep.Targets = endpoint.Targets{dnsRecord.Target}
		default:
			continue
		}

		ep.RecordTTL = defaultTTL
		ep.ProviderSpecific = endpoint.ProviderSpecific{
			{Name: openwrt.UCISectionKey, Value: uciSection},
		}
		endpoints = append(endpoints, &ep)
	}

	return endpoints
}

func endpoints2DNSRecords(endpoints []*endpoint.Endpoint) []openwrt.DNSRecord {
	var dnsRecords []openwrt.DNSRecord

	for _, ep := range endpoints {
		var dnsRecord openwrt.DNSRecord

		switch ep.RecordType {
		case endpoint.RecordTypeA:
			dnsRecord.Type = "A"
			dnsRecord.Name = ep.DNSName
			dnsRecord.IP = ep.Targets[0]
		case endpoint.RecordTypeCNAME:
			dnsRecord.Type = "CNAME"
			dnsRecord.CName = ep.DNSName
			dnsRecord.Target = ep.Targets[0]
		default:
			continue
		}
		dnsRecords = append(dnsRecords, dnsRecord)
	}

	return dnsRecords
}
