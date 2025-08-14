package uci

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Options struct {
	DeviceHostname string
}

// File — тип, который RenderAll собирает в []File и передаёт в tarball.Build
type File struct {
	Name string // путь внутри tar: "etc/config/system"
	Data []byte
	Mode int // 0644 и т.п.; если твоему tarball не нужен — можно оставить 0
}

// RenderAll — рендерит UCI-файлы из NetJSON.
// Поддержка секций: system, network (interfaces/VLAN), wireless, dhcp, firewall,
// и упрощённые блоки wireguard/openvpn/zerotier.
func RenderAll(netjson map[string]any, opts Options) ([]File, error) {
	var files []File

	if f := renderSystem(netjson, opts); f != nil {
		files = append(files, *f)
	}
	if f := renderNetwork(netjson); f != nil {
		files = append(files, *f)
	}
	if f := renderWireless(netjson); f != nil {
		files = append(files, *f)
	}
	if f := renderDHCP(netjson); f != nil {
		files = append(files, *f)
	}
	if f := renderFirewall(netjson); f != nil {
		files = append(files, *f)
	}
	// VPN/overlay
	if f := renderWireGuard(netjson); f != nil {
		files = append(files, *f)
	}
	if f := renderOpenVPN(netjson); f != nil {
		files = append(files, *f)
	}
	if f := renderZeroTier(netjson); f != nil {
		files = append(files, *f)
	}

	return files, nil
}

// ===== helpers =====
func q(s string) string { return strings.ReplaceAll(s, "'", "\\'") }

func addLine(b *strings.Builder, format string, args ...any) { fmt.Fprintf(b, format, args...) }

func opt(b *strings.Builder, k, v string) {
	if v == "" {
		return
	}
	addLine(b, "\toption %s '%s'\n", k, q(v))
}
func optBool(b *strings.Builder, k string, v bool) {
	if v {
		addLine(b, "\toption %s '1'\n", k)
	}
}
func lst(b *strings.Builder, k, v string) {
	if v != "" {
		addLine(b, "\tlist %s '%s'\n", k, q(v))
	}
}

// ===== system =====
func renderSystem(nj map[string]any, opt Options) *File {
	// приоритет: NetJSON > опция > дефолт
	hn := strAt(nj, "system", "hostname")
	if hn == "" {
		hn = opt.DeviceHostname
	}
	if hn == "" {
		hn = "OpenWrt"
	}

	content := fmt.Sprintf(
		"config system\n\toption hostname '%s'\n",
		hn,
	)

	return &File{
		Name: "etc/config/system",
		Data: []byte(content),
		Mode: 0644,
	}
}

// ===== network =====
func renderNetwork(nj map[string]any) *File {
	netw, _ := asMap(nj["network"])
	if netw == nil {
		return nil
	}
	var b strings.Builder
	// interfaces
	ifaces, _ := asSlice(netw["interfaces"]) // []{ name, proto, ipaddr, netmask, gateway, dns[], vlan, disabled }
	for _, it := range ifaces {
		m, _ := asMap(it)
		name := getString(m, "name", "")
		if name == "" {
			continue
		}
		addLine(&b, "config interface '%s'\n", q(name))
		opt(&b, "proto", getString(m, "proto", "static"))
		opt(&b, "ipaddr", getString(m, "ipaddr", ""))
		opt(&b, "netmask", getString(m, "netmask", ""))
		opt(&b, "gateway", getString(m, "gateway", ""))
		dnss, _ := asSlice(m["dns"])
		for _, d := range dnss {
			lst(&b, "dns", fmt.Sprint(d))
		}
		if v := getInt(m, "vlan", 0); v > 0 {
			opt(&b, "ifname", fmt.Sprintf("%s.%d", name, v))
		}
		optBool(&b, "disabled", getBool(m, "disabled", false))
		addLine(&b, "\n")
	}
	return &File{Name: "etc/config/network", Mode: 0644, Data: []byte(b.String())}

}

// ===== wireless =====
func renderWireless(nj map[string]any) *File {
	w, _ := asMap(nj["wireless"])
	if w == nil {
		return nil
	}
	var b strings.Builder
	// devices (radios)
	radios, _ := asSlice(w["radios"]) // []{ name, hwmode, channel, country, disabled }
	for _, r := range radios {
		m, _ := asMap(r)
		name := getString(m, "name", "")
		if name == "" {
			continue
		}
		addLine(&b, "config wifi-device '%s'\n", q(name))
		opt(&b, "type", "mac80211")
		opt(&b, "hwmode", getString(m, "hwmode", ""))
		opt(&b, "channel", getString(m, "channel", "auto"))
		opt(&b, "country", getString(m, "country", ""))
		optBool(&b, "disabled", getBool(m, "disabled", false))
		addLine(&b, "\n")
	}
	// ifaces — плоско в wireless.interfaces
	ifs, _ := asSlice(w["interfaces"]) // []{ device, mode, ssid, encryption, key, network }
	for _, x := range ifs {
		m, _ := asMap(x)
		dev := getString(m, "device", "")
		addLine(&b, "config wifi-iface\n")
		opt(&b, "device", dev)
		opt(&b, "mode", getString(m, "mode", "ap"))
		opt(&b, "ssid", getString(m, "ssid", ""))
		opt(&b, "encryption", getString(m, "encryption", "psk2"))
		opt(&b, "key", getString(m, "key", ""))
		opt(&b, "network", getString(m, "network", "lan"))
		optBool(&b, "disabled", getBool(m, "disabled", false))
		addLine(&b, "\n")
	}
	return &File{Name: "etc/config/wireless", Mode: 0644, Data: []byte(b.String())}
}

// ===== dhcp =====
func renderDHCP(nj map[string]any) *File {
	d, _ := asMap(nj["dhcp"])
	if d == nil {
		return nil
	}
	var b strings.Builder
	servers, _ := asSlice(d["servers"]) // []{ interface, start, limit, leasetime }
	for _, s := range servers {
		m, _ := asMap(s)
		name := getString(m, "interface", "")
		if name == "" {
			continue
		}
		addLine(&b, "config dhcp '%s'\n", q(name))
		opt(&b, "interface", name)
		if v := getInt(m, "start", 0); v > 0 {
			opt(&b, "start", strconv.Itoa(v))
		}
		if v := getInt(m, "limit", 0); v > 0 {
			opt(&b, "limit", strconv.Itoa(v))
		}
		opt(&b, "leasetime", getString(m, "leasetime", "12h"))
		addLine(&b, "\n")
	}
	return &File{Name: "etc/config/dhcp", Mode: 0644, Data: []byte(b.String())}
}

// ===== firewall =====
func renderFirewall(nj map[string]any) *File {
	fw, _ := asMap(nj["firewall"])
	if fw == nil {
		return nil
	}
	var b strings.Builder
	zones, _ := asSlice(fw["zones"]) // []{ name, networks[], input, output, forward }
	for _, z := range zones {
		m, _ := asMap(z)
		name := getString(m, "name", "")
		if name == "" {
			continue
		}
		addLine(&b, "config zone\n")
		opt(&b, "name", name)
		opt(&b, "input", getString(m, "input", "ACCEPT"))
		opt(&b, "output", getString(m, "output", "ACCEPT"))
		opt(&b, "forward", getString(m, "forward", "REJECT"))
		nets, _ := asSlice(m["networks"])
		for _, n := range nets {
			lst(&b, "network", fmt.Sprint(n))
		}
		addLine(&b, "\n")
	}
	rules, _ := asSlice(fw["rules"]) // []{ name, src, dest, proto, dest_port, target, family }
	for i, r := range rules {
		m, _ := asMap(r)
		addLine(&b, "config rule\n")
		opt(&b, "name", getString(m, "name", fmt.Sprintf("rule_%d", i+1)))
		opt(&b, "src", getString(m, "src", ""))
		opt(&b, "dest", getString(m, "dest", ""))
		opt(&b, "proto", getString(m, "proto", "tcpudp"))
		opt(&b, "target", getString(m, "target", "ACCEPT"))
		if p := getString(m, "dest_port", ""); p != "" {
			opt(&b, "dest_port", p)
		}
		if f := getString(m, "family", ""); f != "" {
			opt(&b, "family", f)
		}
		addLine(&b, "\n")
	}
	return &File{Name: "etc/config/firewall", Mode: 0644, Data: []byte(b.String())}

}

// ===== WireGuard =====
func renderWireGuard(nj map[string]any) *File {
	wg, _ := asMap(nj["wireguard"]) // { interface, address, private_key, peers[] }
	if wg == nil {
		return nil
	}
	var b strings.Builder
	iface := getString(wg, "interface", "wg0")
	addLine(&b, "config interface '%s'\n", q(iface))
	opt(&b, "proto", "wireguard")
	opt(&b, "private_key", getString(wg, "private_key", ""))
	if addr := getString(wg, "address", ""); addr != "" {
		lst(&b, "addresses", addr)
	}
	peers, _ := asSlice(wg["peers"]) // []map
	for _, p := range peers {
		m, _ := asMap(p)
		addLine(&b, "config wireguard_%s\n", q(iface))
		opt(&b, "public_key", getString(m, "public_key", ""))
		if v := getString(m, "preshared_key", ""); v != "" {
			opt(&b, "preshared_key", v)
		}
		if ep := getString(m, "endpoint", ""); ep != "" {
			h, port, _ := net.SplitHostPort(ep)
			opt(&b, "endpoint_host", h)
			opt(&b, "endpoint_port", port)
		}
		ips, _ := asSlice(m["allowed_ips"])
		for _, ip := range ips {
			lst(&b, "allowed_ips", fmt.Sprint(ip))
		}
		if ka := getInt(m, "keepalive", 0); ka > 0 {
			opt(&b, "persistent_keepalive", strconv.Itoa(ka))
		}
		addLine(&b, "\n")
	}
	return &File{Name: "etc/config/network", Mode: 0644, Data: []byte(b.String())}
}

// ===== OpenVPN =====
func renderOpenVPN(nj map[string]any) *File {
	ov, _ := asMap(nj["openvpn"]) // { clients:[{ name, remote, port, proto, cipher, auth, config_file }] }
	if ov == nil {
		return nil
	}
	var b strings.Builder
	clients, _ := asSlice(ov["clients"])
	for _, c := range clients {
		m, _ := asMap(c)
		name := getString(m, "name", "client")
		addLine(&b, "config openvpn '%s'\n", q(name))
		opt(&b, "enabled", "1")
		opt(&b, "client", "1")
		if host := getString(m, "remote", ""); host != "" {
			port := getInt(m, "port", 1194)
			opt(&b, "remote", fmt.Sprintf("%s %d", host, port))
		}
		opt(&b, "proto", getString(m, "proto", "udp"))
		opt(&b, "cipher", getString(m, "cipher", "AES-256-GCM"))
		opt(&b, "auth", getString(m, "auth", "SHA256"))
		if cf := getString(m, "config_file", ""); cf != "" {
			opt(&b, "config", cf)
		}
		addLine(&b, "\n")
	}
	return &File{Name: "etc/config/openvpn", Mode: 0644, Data: []byte(b.String())}
}

// ===== ZeroTier =====
func renderZeroTier(nj map[string]any) *File {
	zt, _ := asMap(nj["zerotier"]) // { enabled: true, networks: ["<id>", ...] }
	if zt == nil {
		return nil
	}
	var b strings.Builder
	addLine(&b, "config zerotier\n")
	optBool(&b, "enabled", getBool(zt, "enabled", true))
	nets, _ := asSlice(zt["networks"])
	for _, n := range nets {
		lst(&b, "join", fmt.Sprint(n))
	}
	addLine(&b, "\n")
	return &File{Name: "etc/config/zerotier", Mode: 0644, Data: []byte(b.String())}
}

// ===== small helpers =====
func asMap(v any) (map[string]any, bool) { m, ok := v.(map[string]any); return m, ok }
func asSlice(v any) ([]any, bool)        { s, ok := v.([]any); return s, ok }

func getString(m map[string]any, path ...string) string {
	cur := any(m)
	for _, p := range path {
		obj, _ := cur.(map[string]any)
		if obj == nil {
			return ""
		}
		cur = obj[p]
	}
	s, _ := cur.(string)
	return s
}

func getBool(m map[string]any, k string, def bool) bool {
	if m == nil {
		return def
	}
	if v, ok := m[k]; ok {
		switch vv := v.(type) {
		case bool:
			return vv
		case string:
			return vv == "1" || strings.EqualFold(vv, "true")
		case float64:
			return vv != 0
		}
	}
	return def
}
func getInt(m map[string]any, k string, def int) int {
	if m == nil {
		return def
	}
	if v, ok := m[k]; ok {
		switch vv := v.(type) {
		case int:
			return vv
		case float64:
			return int(vv)
		case string:
			if i, err := strconv.Atoi(vv); err == nil {
				return i
			}
		}
	}
	return def
}

// strAt — безопасно достаёт строку из вложенной map[string]any по пути ключей
func strAt(m map[string]any, keys ...string) string {
	cur := any(m)
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = obj[k]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}
