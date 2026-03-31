package openwrt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yukariin/external-dns-openwrt-webhook/pkg/logger"
	"github.com/yukariin/external-dns-openwrt-webhook/pkg/lucirpc"
	"go.uber.org/zap"
)

//go:generate mockgen -destination=../../internal/mocks/openwrt/openwrt.go -package=mocks . OpenWRT

type OpenWRT interface {
	GetDNSRecords(context.Context) (map[string]DNSRecord, error)
	SetDNSRecords(context.Context, []DNSRecord) error
	DeleteDNSRecords(context.Context, []DNSRecord) error
}

type openWRT struct {
	lucirpc lucirpc.LuciRPC
}

func New(cfg *Config) (OpenWRT, error) {
	lrcp, err := lucirpc.New(cfg.LuciRPC)
	if err != nil {
		return nil, err
	}

	return &openWRT{
		lucirpc: lrcp,
	}, nil
}

func (o *openWRT) GetDNSRecords(ctx context.Context) (map[string]DNSRecord, error) {
	result, err := o.lucirpc.Uci(ctx, "get_all", []string{"dhcp"})
	if err != nil {
		return nil, err
	}

	if result == "" {
		return make(map[string]DNSRecord), nil
	}

	var records map[string]DNSRecord
	err = json.Unmarshal([]byte(result), &records)
	if err != nil {
		return nil, err
	}

	for key, record := range records {
		switch record.Type {
		case "domain":
			records[key] = DNSRecord{
				Type: "A",
				IP:   record.IP,
				Name: record.Name,
			}
		case "cname":
			records[key] = DNSRecord{
				Type:   "CNAME",
				CName:  record.CName,
				Target: record.Target,
			}
		case "txt":
			records[key] = DNSRecord{
				Type:  "TXT",
				Name:  record.Name,
				Value: record.Value,
			}
		default:
			// it does not care about other types
			logger.Log.Debug("ignoring record", zap.String("type", record.Type))
			delete(records, key)
		}
	}

	logger.Log.Debug("current records", zap.Any("records", records))
	return records, nil
}

func (o *openWRT) SetDNSRecords(ctx context.Context, records []DNSRecord) error {
	for _, record := range records {
		switch record.Type {
		case "A":
			if err := o.addA(ctx, record); err != nil {
				return err
			}
		case "CNAME":
			if err := o.addCName(ctx, record); err != nil {
				return err
			}
		case "TXT":
			if err := o.addTXT(ctx, record); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid record type: %s", record.Type)
		}
	}

	if _, err := o.lucirpc.Uci(ctx, "commit", []string{"dhcp"}); err != nil {
		return err
	}
	logger.Log.Debug("set records", zap.Any("records", records))

	return nil
}

func (o *openWRT) DeleteDNSRecords(ctx context.Context, deleteRecords []DNSRecord) error {
	currentRecords, err := o.GetDNSRecords(ctx)
	if err != nil {
		return err
	}

	matched := make(map[int]bool)

	for cfg, currentRecord := range currentRecords {
		for i, deleteRecord := range deleteRecords {
			if matched[i] {
				continue
			}
			if recordMatches(currentRecord, deleteRecord) {
				if _, err := o.lucirpc.Uci(ctx, "delete", []string{"dhcp", cfg}); err != nil {
					return err
				}
				logger.Log.Debug("deleted record", zap.String("section", cfg), zap.Any("record", currentRecord))
				matched[i] = true
				break
			}
		}
	}

	var unmatched []DNSRecord
	for i, rec := range deleteRecords {
		if !matched[i] {
			unmatched = append(unmatched, rec)
		}
	}
	if len(unmatched) > 0 {
		return fmt.Errorf("records not found for deletion: %v", unmatched)
	}

	if _, err := o.lucirpc.Uci(ctx, "commit", []string{"dhcp"}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) addA(ctx context.Context, record DNSRecord) error {
	if record.Type != "A" {
		return fmt.Errorf("invalid record type: %s", record.Type)
	}

	if record.Name == "" {
		return fmt.Errorf("name is required")
	}

	if record.IP == "" {
		return fmt.Errorf("ip is required")
	}

	cfg, err := o.lucirpc.Uci(ctx, "add", []string{"dhcp", "domain"})
	if err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "name", record.Name}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "ip", record.IP}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) addCName(ctx context.Context, record DNSRecord) error {
	if record.Type != "CNAME" {
		return fmt.Errorf("invalid record type: %s", record.Type)
	}

	if record.CName == "" {
		return fmt.Errorf("cname is required")
	}

	if record.Target == "" {
		return fmt.Errorf("target is required")
	}

	cfg, err := o.lucirpc.Uci(ctx, "add", []string{"dhcp", "cname"})
	if err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "cname", record.CName}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "target", record.Target}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) addTXT(ctx context.Context, record DNSRecord) error {
	if record.Type != "TXT" {
		return fmt.Errorf("invalid record type: %s", record.Type)
	}

	if record.Name == "" {
		return fmt.Errorf("name is required")
	}

	if record.Value == "" {
		return fmt.Errorf("value is required")
	}

	cfg, err := o.lucirpc.Uci(ctx, "add", []string{"dhcp", "txt"})
	if err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "name", record.Name}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "value", record.Value}); err != nil {
		return err
	}

	return nil
}

// recordMatches checks whether a current UCI record matches a desired record
// by comparing type, name, and value (IP for A records, target for CNAME, value for TXT).
func recordMatches(current DNSRecord, desired DNSRecord) bool {
	switch desired.Type {
	case "A":
		return current.Type == "A" && current.Name == desired.Name && current.IP == desired.IP
	case "CNAME":
		return current.Type == "CNAME" && current.CName == desired.CName && current.Target == desired.Target
	case "TXT":
		return current.Type == "TXT" && current.Name == desired.Name && current.Value == desired.Value
	}
	return false
}
