package controller

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"wisp/config"
	"wisp/internal/models"
	"wisp/internal/pki"
	rnetjson "wisp/internal/render/netjson" // ← добавили алиас
	"wisp/internal/render/uci"
	"wisp/internal/repo"
	"wisp/internal/tarball"
	"wisp/internal/vpn/wireguard"
)

// Репозитории
type DeviceRepo interface {
	GetByUUID(ctx context.Context, uuid string) (*models.Device, error)
	PutConfigTar(ctx context.Context, uuid string, tarGz []byte, version int) error
	EnsureWGPeer(ctx context.Context, deviceID uint, newPeer func() (*models.WireGuardPeer, error)) (*models.WireGuardPeer, error)
}
type Templates interface {
	ListForDevice(ctx context.Context, deviceID uint) ([]models.ConfigTemplate, error)
	VarsForDevice(ctx context.Context, dev *models.Device) (map[string]any, error)
}

type Reconciler struct {
	Devices   DeviceRepo
	Templates Templates
	PKI       *pki.Service
	Cfg       *config.Config
}

func NewReconciler(ds DeviceRepo, ts Templates, pkiSvc *pki.Service, cfg *config.Config) *Reconciler {
	return &Reconciler{Devices: ds, Templates: ts, PKI: pkiSvc, Cfg: cfg}
}

func (r *Reconciler) Reconcile(ctx context.Context, uuid string) (checksum string, updated bool, err error) {
	dev, err := r.Devices.GetByUUID(ctx, uuid)
	if err != nil || dev == nil {
		return "", false, err
	}

	// 1) merge NetJSON шаблонов
	tpls, err := r.Templates.ListForDevice(ctx, dev.ID)
	if err != nil {
		return "", false, err
	}
	sources := make([]rnetjson.Source, 0, len(tpls)+1)
	for _, t := range tpls {
		m, err := repo.DecodeNetJSON(t)
		if err != nil {
			return "", false, err
		}
		sources = append(sources, rnetjson.Source{Name: t.Name, Priority: t.Priority, JSON: m})
	}
	merged, err := rnetjson.Merge(sources...)
	if err != nil {
		return "", false, err
	}

	// 2) vars
	vars, err := r.Templates.VarsForDevice(ctx, dev)
	if err != nil {
		return "", false, err
	}
	merged, err = rnetjson.ApplyVars(merged, vars)
	if err != nil {
		return "", false, err
	}

	// 3) VPN/PKI overlays + extra files
	extra := map[string][]byte{}
	const ifaceWG = "wg0"

	switch strings.ToLower(r.Cfg.OpenWISP.Controller.MgmtVPN.Mode) {
	case "wireguard":
		overlay, err := r.overlayWireGuard(ctx, dev)
		if err != nil {
			return "", false, err
		}
		merged, _ = rnetjson.Merge(
			rnetjson.Source{Name: "base", Priority: 10, JSON: merged},
			rnetjson.Source{Name: "wg", Priority: 999, JSON: overlay},
		)
		// (опционально) сгенерить wg0.conf
		if conf := buildWGConf(overlay); conf != nil {
			extra["etc/wireguard/wg0.conf"] = conf
		}

	case "openvpn":
		overlay := overlayOpenVPN(r.Cfg)
		merged, _ = rnetjson.Merge(
			rnetjson.Source{Name: "base", Priority: 10, JSON: merged},
			rnetjson.Source{Name: "ovpn", Priority: 999, JSON: overlay},
		)
		// PKI: выпустим сертификат и сложим файлы
		caTTL, _ := time.ParseDuration(zeroIfEmpty(r.Cfg.OpenWISP.Controller.PKI.CertTTL, "8760h"))
		ca, err := r.PKI.EnsureRootCA(ctx, zeroIfEmpty(r.Cfg.OpenWISP.Controller.PKI.CAName, "OpenWISP-Go-CA"), caTTL)
		if err != nil {
			return "", false, err
		}
		cert, err := r.PKI.IssueDeviceCert(ctx, ca, dev.UUID, caTTL, &dev.ID)
		if err != nil {
			return "", false, err
		}
		dir := fmt.Sprintf("etc/openvpn/%s/", dev.UUID)
		extra[dir+"ca.crt"] = ca.CertPEM
		extra[dir+"client.crt"] = cert.CertPEM
		extra[dir+"client.key"] = cert.KeyPEM

	case "zerotier":
		overlay := overlayZeroTier(r.Cfg)
		merged, _ = rnetjson.Merge(
			rnetjson.Source{Name: "base", Priority: 10, JSON: merged},
			rnetjson.Source{Name: "zt", Priority: 999, JSON: overlay},
		)
	}

	// 4) UCI → tar.gz
	files, err := uci.RenderAll(merged, uci.Options{DeviceHostname: dev.Name})
	if err != nil {
		return "", false, err
	}
	tarGz, sum, err := tarball.Build(files, extra)
	if err != nil {
		return "", false, err
	}

	if sum == dev.ConfigChecksum {
		return sum, false, nil
	}
	ver := dev.ConfigVersion + 1
	if ver <= 0 {
		ver = 1
	}
	if err := r.Devices.PutConfigTar(ctx, uuid, tarGz, ver); err != nil {
		return "", false, err
	}
	return sum, true, nil
}

// ---- overlays ----

func (r *Reconciler) overlayWireGuard(ctx context.Context, dev *models.Device) (map[string]any, error) {
	cfg := r.Cfg.OpenWISP.Controller.MgmtVPN.WireGuard
	// адрес из пула: простейший генератор /32 по Device.ID
	addr, err := pickWGAddress(cfg.AddressPoolCIDR, dev.ID)
	if err != nil {
		return nil, err
	}

	peer, err := r.Devices.EnsureWGPeer(ctx, dev.ID, func() (*models.WireGuardPeer, error) {
		return wireguard.GeneratePeer(addr, cfg.ServerPublicKey, cfg.Endpoint, cfg.AllowedIPs, cfg.Keepalive)
	})
	if err != nil {
		return nil, err
	}

	ov := map[string]any{
		"wireguard": map[string]any{
			"interface":   "wg0",
			"address":     peer.AddressCIDR,
			"private_key": peer.PrivateKey,
			"peers": []any{
				map[string]any{
					"public_key":    peer.ServerPub,
					"preshared_key": peer.PresharedKey,
					"endpoint":      cfg.Endpoint,
					"allowed_ips":   cfg.AllowedIPs,
					"keepalive":     cfg.Keepalive,
				},
			},
		},
	}
	return ov, nil
}

func overlayOpenVPN(cfg *config.Config) map[string]any {
	ov := cfg.OpenWISP.Controller.MgmtVPN.OpenVPN
	return map[string]any{
		"openvpn": map[string]any{
			"clients": []any{
				map[string]any{
					"name":   "client",
					"remote": ov.Remote,
					"port":   ov.Port,
					"proto":  ov.Proto,
					"cipher": ov.Cipher,
					"auth":   ov.Auth,
					// можно указать config_file, если хотите отдельный .conf
				},
			},
		},
	}
}

func overlayZeroTier(cfg *config.Config) map[string]any {
	return map[string]any{
		"zerotier": map[string]any{
			"enabled":  true,
			"networks": []any{cfg.OpenWISP.Controller.MgmtVPN.ZeroTier.NetworkID},
		},
	}
}

// ---- helpers ----

func pickWGAddress(cidr string, deviceID uint) (string, error) {
	// простой /24 пул: base + deviceID (не для прод-ISP, но достаточно для MVP)
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}
	base := ipnet.IP.To4()
	if base == nil {
		return "", fmt.Errorf("only IPv4 supported for AddressPoolCIDR")
	}
	// + deviceID, начиная с .2
	host := 2 + int(deviceID%250)
	ip := net.IPv4(base[0], base[1], base[2], byte(host))
	return ip.String() + "/32", nil
}

func buildWGConf(overlay map[string]any) []byte {
	w, _ := overlay["wireguard"].(map[string]any)
	if w == nil {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\n")
	if addr, _ := w["address"].(string); addr != "" {
		fmt.Fprintf(&b, "Address = %s\n", addr)
	}
	if pk, _ := w["private_key"].(string); pk != "" {
		fmt.Fprintf(&b, "PrivateKey = %s\n", pk)
	}
	peers, _ := w["peers"].([]any)
	for _, p := range peers {
		m, _ := p.(map[string]any)
		fmt.Fprintf(&b, "\n[Peer]\n")
		if v, _ := m["public_key"].(string); v != "" {
			fmt.Fprintf(&b, "PublicKey = %s\n", v)
		}
		if v, _ := m["preshared_key"].(string); v != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", v)
		}
		if ep, _ := m["endpoint"].(string); ep != "" {
			fmt.Fprintf(&b, "Endpoint = %s\n", ep)
		}
		if a, _ := m["allowed_ips"].([]string); len(a) > 0 {
			fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(a, ",")) // для простоты
		}
		if ka, _ := m["keepalive"].(int); ka > 0 {
			fmt.Fprintf(&b, "PersistentKeepalive = %d\n", ka)
		}
	}
	return []byte(b.String())
}

func zeroIfEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
