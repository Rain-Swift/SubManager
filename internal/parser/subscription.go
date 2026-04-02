package parser

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"submanager/internal/domain"
)

type SubscriptionParseResult struct {
	RawProxies []domain.RawProxyIR
	Proxies    []domain.ProxyIR
	Warnings   []string
}

type clashConfig struct {
	Proxies []map[string]any `yaml:"proxies"`
}

func ParseClashMetaSubscription(body []byte, sourceID, sourceURL string) (SubscriptionParseResult, error) {
	var cfg clashConfig
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return SubscriptionParseResult{}, fmt.Errorf("parse subscription yaml: %w", err)
	}
	if len(cfg.Proxies) == 0 {
		return SubscriptionParseResult{}, fmt.Errorf("parse subscription yaml: no proxies field found")
	}

	result := SubscriptionParseResult{
		RawProxies: make([]domain.RawProxyIR, 0, len(cfg.Proxies)),
		Proxies:    make([]domain.ProxyIR, 0, len(cfg.Proxies)),
	}

	for index, item := range cfg.Proxies {
		name := getString(item, "name")
		if name == "" {
			name = fmt.Sprintf("unnamed-%d", index+1)
		}
		proxyType := strings.ToLower(getString(item, "type"))

		result.RawProxies = append(result.RawProxies, domain.RawProxyIR{
			Index:     index,
			Name:      name,
			Type:      proxyType,
			SourceURL: sourceURL,
			Original:  item,
		})

		proxy, warning, ok := normalizeProxy(item, index, sourceID, sourceURL)
		if warning != "" {
			result.Warnings = append(result.Warnings, warning)
		}
		if ok {
			result.Proxies = append(result.Proxies, proxy)
		}
	}

	return result, nil
}

func normalizeProxy(item map[string]any, index int, sourceID, sourceURL string) (domain.ProxyIR, string, bool) {
	name := getString(item, "name")
	if name == "" {
		name = fmt.Sprintf("unnamed-%d", index+1)
	}
	proxyType := strings.ToLower(getString(item, "type"))
	server := getString(item, "server")
	port := getInt(item, "port")
	uuid := getString(item, "uuid")
	password := getString(item, "password")
	cipher := getString(item, "cipher")
	network := getString(item, "network")
	path := firstNonEmpty(
		getString(item, "ws-opts", "path"),
		getString(item, "http-opts", "path"),
		getString(item, "h2-opts", "path"),
	)
	host := firstNonEmpty(
		getString(item, "ws-opts", "headers", "Host"),
		getString(item, "ws-opts", "headers", "host"),
		getString(item, "http-opts", "host"),
	)
	sni := firstNonEmpty(getString(item, "servername"), getString(item, "sni"))
	alpn := getStringSlice(item, "alpn")
	tls := normalizeTLS(proxyType, item)
	endpointFingerprint := fingerprintProxyFields(
		proxyType,
		server,
		port,
		uuid,
		password,
		network,
		tls,
		sni,
		path,
	)

	if proxyType == "" {
		return domain.ProxyIR{}, fmt.Sprintf("skip proxy[%d]: missing type", index), false
	}
	if server == "" || port == 0 {
		return domain.ProxyIR{}, fmt.Sprintf("skip proxy[%d]: missing server or port", index), false
	}

	proxy := domain.ProxyIR{
		Name:         name,
		OriginalName: getString(item, "name"),
		Type:         proxyType,
		Server:       server,
		Port:         port,
		UUID:         uuid,
		Password:     password,
		Cipher:       cipher,
		TLS:          tls,
		UDP:          getBool(item, "udp"),
		Network:      network,
		SNI:          sni,
		Host:         host,
		Path:         path,
		SourceID:     sourceID,
		SourceURL:    sourceURL,
		Metadata: map[string]string{
			"source_id":            sourceID,
			"source_url":           sourceURL,
			"original_name":        getString(item, "name"),
			"source_index":         strconv.Itoa(index),
			"endpoint_fingerprint": endpointFingerprint,
		},
	}
	proxy.ID = buildProxyRecordID(sourceID, index, proxy)
	proxy.VLESSOptions = buildVLESSOptions(proxyType, item, alpn)
	proxy.TUICOptions = buildTUICOptions(proxyType, item, alpn)
	proxy.Hysteria2Options = buildHysteria2Options(proxyType, item, alpn)
	return proxy, "", true
}

func buildVLESSOptions(proxyType string, item map[string]any, alpn []string) *domain.VLESSOptions {
	if proxyType != "vless" {
		return nil
	}

	options := &domain.VLESSOptions{
		Flow:              getString(item, "flow"),
		PacketEncoding:    getString(item, "packet-encoding"),
		Encryption:        getString(item, "encryption"),
		SkipCertVerify:    getBool(item, "skip-cert-verify"),
		Fingerprint:       getString(item, "fingerprint"),
		ClientFingerprint: getString(item, "client-fingerprint"),
		ALPN:              alpn,
	}

	if reality := buildRealityOptions(item); reality != nil {
		options.RealityOptions = reality
	}
	if smux := buildSmuxOptions(item); smux != nil {
		options.SmuxOptions = smux
	}

	return options
}

func buildTUICOptions(proxyType string, item map[string]any, alpn []string) *domain.TUICOptions {
	if proxyType != "tuic" {
		return nil
	}

	return &domain.TUICOptions{
		Token:                 getString(item, "token"),
		IP:                    getString(item, "ip"),
		HeartbeatIntervalMS:   getInt(item, "heartbeat-interval"),
		DisableSNI:            getBool(item, "disable-sni"),
		ReduceRTT:             getBool(item, "reduce-rtt"),
		RequestTimeoutMS:      getInt(item, "request-timeout"),
		UDPRelayMode:          getString(item, "udp-relay-mode"),
		CongestionController:  getString(item, "congestion-controller"),
		MaxUDPRelayPacketSize: getInt(item, "max-udp-relay-packet-size"),
		FastOpen:              getBool(item, "fast-open"),
		MaxOpenStreams:        getInt(item, "max-open-streams"),
		SkipCertVerify:        getBool(item, "skip-cert-verify"),
		ALPN:                  alpn,
	}
}

func buildHysteria2Options(proxyType string, item map[string]any, alpn []string) *domain.Hysteria2Options {
	if proxyType != "hysteria2" {
		return nil
	}

	return &domain.Hysteria2Options{
		Ports:                          getString(item, "ports"),
		HopIntervalSec:                 getInt(item, "hop-interval"),
		Up:                             getString(item, "up"),
		Down:                           getString(item, "down"),
		Obfs:                           getString(item, "obfs"),
		ObfsPassword:                   getString(item, "obfs-password"),
		SkipCertVerify:                 getBool(item, "skip-cert-verify"),
		Fingerprint:                    getString(item, "fingerprint"),
		ALPN:                           alpn,
		InitialStreamReceiveWindow:     getInt(item, "initial-stream-receive-window"),
		MaxStreamReceiveWindow:         getInt(item, "max-stream-receive-window"),
		InitialConnectionReceiveWindow: getInt(item, "initial-connection-receive-window"),
		MaxConnectionReceiveWindow:     getInt(item, "max-connection-receive-window"),
	}
}

func buildRealityOptions(item map[string]any) *domain.RealityOptions {
	if !hasValue(item, "reality-opts") {
		return nil
	}

	return &domain.RealityOptions{
		PublicKey:             getString(item, "reality-opts", "public-key"),
		ShortID:               getString(item, "reality-opts", "short-id"),
		SupportX25519MLKEM768: getBool(item, "reality-opts", "support-x25519mlkem768"),
	}
}

func buildSmuxOptions(item map[string]any) *domain.SmuxOptions {
	if !hasValue(item, "smux") {
		return nil
	}

	return &domain.SmuxOptions{
		Enabled: getBool(item, "smux", "enabled"),
	}
}

func fingerprintProxy(proxy domain.ProxyIR) string {
	return fingerprintProxyFields(
		proxy.Type,
		proxy.Server,
		proxy.Port,
		proxy.UUID,
		proxy.Password,
		proxy.Network,
		proxy.TLS,
		proxy.SNI,
		proxy.Path,
	)
}

func fingerprintProxyFields(proxyType, server string, port int, uuid, password, network string, tls bool, sni, path string) string {
	hash := sha1.Sum([]byte(strings.Join([]string{
		proxyType,
		server,
		strconv.Itoa(port),
		uuid,
		password,
		network,
		strconv.FormatBool(tls),
		sni,
		path,
	}, "|")))
	return hex.EncodeToString(hash[:])
}

func buildProxyRecordID(sourceID string, index int, proxy domain.ProxyIR) string {
	hash := sha1.Sum([]byte(strings.Join([]string{
		sourceID,
		strconv.Itoa(index),
		fingerprintProxy(proxy),
	}, "|")))
	return hex.EncodeToString(hash[:])
}

func normalizeTLS(proxyType string, item map[string]any) bool {
	if hasValue(item, "tls") {
		return getBool(item, "tls")
	}

	// Trojan transport is TLS-based in Clash/Mihomo configs unless explicitly disabled.
	if proxyType == "trojan" {
		return true
	}

	return false
}

func getString(root map[string]any, keys ...string) string {
	value, ok := getValue(root, keys...)
	if !ok || value == nil {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return typed
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.Itoa(int(typed))
	case bool:
		return strconv.FormatBool(typed)
	case []any:
		if len(typed) == 0 {
			return ""
		}
		return stringifyValue(typed[0])
	default:
		return ""
	}
}

func getInt(root map[string]any, keys ...string) int {
	value, ok := getValue(root, keys...)
	if !ok || value == nil {
		return 0
	}

	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func getBool(root map[string]any, keys ...string) bool {
	value, ok := getValue(root, keys...)
	if !ok || value == nil {
		return false
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func getStringSlice(root map[string]any, keys ...string) []string {
	value, ok := getValue(root, keys...)
	if !ok || value == nil {
		return nil
	}

	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringifyValue(item); text != "" {
				out = append(out, text)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []string:
		out := make([]string, len(typed))
		copy(out, typed)
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func getValue(root map[string]any, keys ...string) (any, bool) {
	current := any(root)
	for _, key := range keys {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, exists := object[key]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func hasValue(root map[string]any, keys ...string) bool {
	_, ok := getValue(root, keys...)
	return ok
}

func stringifyValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.Itoa(int(typed))
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
