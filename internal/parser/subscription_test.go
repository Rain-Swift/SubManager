package parser

import "testing"

func TestParseClashMetaSubscriptionProtocolOptions(t *testing.T) {
	body := []byte(`
proxies:
  - name: vless-reality
    type: vless
    server: vless.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    flow: xtls-rprx-vision
    packet-encoding: xudp
    tls: true
    servername: edge.example.com
    alpn:
      - h2
      - http/1.1
    fingerprint: sha256:vless
    client-fingerprint: chrome
    skip-cert-verify: true
    reality-opts:
      public-key: reality-public
      short-id: abcd1234
      support-x25519mlkem768: true
    encryption: ""
    smux:
      enabled: true

  - name: tuic-v5
    type: tuic
    server: tuic.example.com
    port: 10443
    uuid: 22222222-2222-2222-2222-222222222222
    password: secret
    ip: 127.0.0.1
    heartbeat-interval: 10000
    alpn: [h3]
    disable-sni: true
    reduce-rtt: true
    request-timeout: 8000
    udp-relay-mode: native
    congestion-controller: bbr
    max-udp-relay-packet-size: 1500
    fast-open: true
    skip-cert-verify: true
    max-open-streams: 20

  - name: hysteria2
    type: hysteria2
    server: hy2.example.com
    port: 443
    ports: 443-8443
    hop-interval: 30
    password: hy2-secret
    up: "30 Mbps"
    down: "200 Mbps"
    obfs: salamander
    obfs-password: hy2-obfs
    sni: hy2.example.com
    skip-cert-verify: true
    fingerprint: sha256:hy2
    alpn:
      - h3
    initial-stream-receive-window: 8388608
    max-stream-receive-window: 8388608
    initial-connection-receive-window: 20971520
    max-connection-receive-window: 20971520
`)

	result, err := ParseClashMetaSubscription(body, "sub_test", "https://example.com/sub.yaml")
	if err != nil {
		t.Fatalf("ParseClashMetaSubscription() error = %v", err)
	}

	if got, want := len(result.Proxies), 3; got != want {
		t.Fatalf("len(result.Proxies) = %d, want %d", got, want)
	}

	vless := result.Proxies[0]
	if vless.VLESSOptions == nil {
		t.Fatalf("vless.VLESSOptions = nil")
	}
	if vless.VLESSOptions.Flow != "xtls-rprx-vision" {
		t.Fatalf("vless flow = %q, want %q", vless.VLESSOptions.Flow, "xtls-rprx-vision")
	}
	if !vless.VLESSOptions.SkipCertVerify {
		t.Fatalf("vless skip-cert-verify = false, want true")
	}
	if len(vless.VLESSOptions.ALPN) != 2 || vless.VLESSOptions.ALPN[0] != "h2" {
		t.Fatalf("vless alpn = %#v, want [h2 http/1.1]", vless.VLESSOptions.ALPN)
	}
	if vless.VLESSOptions.RealityOptions == nil {
		t.Fatalf("vless reality options = nil")
	}
	if vless.VLESSOptions.RealityOptions.PublicKey != "reality-public" {
		t.Fatalf("vless reality public-key = %q, want %q", vless.VLESSOptions.RealityOptions.PublicKey, "reality-public")
	}
	if !vless.VLESSOptions.RealityOptions.SupportX25519MLKEM768 {
		t.Fatalf("vless reality support-x25519mlkem768 = false, want true")
	}
	if vless.VLESSOptions.SmuxOptions == nil || !vless.VLESSOptions.SmuxOptions.Enabled {
		t.Fatalf("vless smux enabled = false, want true")
	}

	tuic := result.Proxies[1]
	if tuic.TUICOptions == nil {
		t.Fatalf("tuic.TUICOptions = nil")
	}
	if tuic.TUICOptions.HeartbeatIntervalMS != 10000 {
		t.Fatalf("tuic heartbeat-interval = %d, want 10000", tuic.TUICOptions.HeartbeatIntervalMS)
	}
	if tuic.TUICOptions.RequestTimeoutMS != 8000 {
		t.Fatalf("tuic request-timeout = %d, want 8000", tuic.TUICOptions.RequestTimeoutMS)
	}
	if tuic.TUICOptions.CongestionController != "bbr" {
		t.Fatalf("tuic congestion-controller = %q, want %q", tuic.TUICOptions.CongestionController, "bbr")
	}
	if !tuic.TUICOptions.FastOpen {
		t.Fatalf("tuic fast-open = false, want true")
	}

	hy2 := result.Proxies[2]
	if hy2.Hysteria2Options == nil {
		t.Fatalf("hy2.Hysteria2Options = nil")
	}
	if hy2.Hysteria2Options.Ports != "443-8443" {
		t.Fatalf("hy2 ports = %q, want %q", hy2.Hysteria2Options.Ports, "443-8443")
	}
	if hy2.Hysteria2Options.Obfs != "salamander" {
		t.Fatalf("hy2 obfs = %q, want %q", hy2.Hysteria2Options.Obfs, "salamander")
	}
	if hy2.Hysteria2Options.MaxConnectionReceiveWindow != 20971520 {
		t.Fatalf("hy2 max-connection-receive-window = %d, want 20971520", hy2.Hysteria2Options.MaxConnectionReceiveWindow)
	}
}
