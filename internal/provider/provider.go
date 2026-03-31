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
	endpoints := make([]*endpoint.Endpoint, 0, len(dnsRecords))
	aByDNSName := make(map[string]*endpoint.Endpoint)

	for _, dnsRecord := range dnsRecords {

		switch dnsRecord.Type {
		case "A":
			if ep, ok := aByDNSName[dnsRecord.Name]; ok {
				ep.Targets = append(ep.Targets, dnsRecord.IP)
			} else {
				ep := &endpoint.Endpoint{
					DNSName:    dnsRecord.Name,
					RecordType: endpoint.RecordTypeA,
					RecordTTL:  defaultTTL,
					Targets:    endpoint.Targets{dnsRecord.IP},
				}
				aByDNSName[dnsRecord.Name] = ep
				endpoints = append(endpoints, ep)
			}
		case "CNAME":
			endpoints = append(endpoints, &endpoint.Endpoint{
				DNSName:    dnsRecord.CName,
				RecordType: endpoint.RecordTypeCNAME,
				RecordTTL:  defaultTTL,
				Targets:    endpoint.Targets{dnsRecord.Target},
			})
		case "TXT":
			endpoints = append(endpoints, &endpoint.Endpoint{
				DNSName:    dnsRecord.Name,
				RecordType: endpoint.RecordTypeTXT,
				RecordTTL:  defaultTTL,
				Targets:    endpoint.Targets{dnsRecord.Value},
			})
		default:
			logger.Log.Warn("skipping unknown DNS record type", zap.String("type", dnsRecord.Type))
		}
	}

	return endpoints
}

func endpoints2DNSRecords(endpoints []*endpoint.Endpoint) []openwrt.DNSRecord {
	var dnsRecords []openwrt.DNSRecord

	for _, ep := range endpoints {
		if len(ep.Targets) == 0 {
			logger.Log.Warn("skipping endpoint with no targets", zap.String("dnsName", ep.DNSName), zap.String("type", ep.RecordType))
			continue
		}

		switch ep.RecordType {
		case endpoint.RecordTypeA:
			for _, target := range ep.Targets {
				dnsRecords = append(dnsRecords, openwrt.DNSRecord{
					Type: "A",
					Name: ep.DNSName,
					IP:   target,
				})
			}
		case endpoint.RecordTypeCNAME:
			dnsRecords = append(dnsRecords, openwrt.DNSRecord{
				Type:   "CNAME",
				CName:  ep.DNSName,
				Target: ep.Targets[0],
			})
		case endpoint.RecordTypeTXT:
			dnsRecords = append(dnsRecords, openwrt.DNSRecord{
				Type:  "TXT",
				Name:  ep.DNSName,
				Value: ep.Targets[0],
			})
		default:
			logger.Log.Warn("skipping unsupported endpoint record type", zap.String("dnsName", ep.DNSName), zap.String("type", ep.RecordType))
		}
	}

	return dnsRecords
}
