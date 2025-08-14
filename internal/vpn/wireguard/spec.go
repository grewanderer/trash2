package wireguard

import (
	"strings"

	"wisp/internal/models"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func GeneratePeer(addressCIDR, serverPub, endpoint string, allowed []string, keepalive int) (*models.WireGuardPeer, error) {
	priv, _ := wgtypes.GeneratePrivateKey()
	psk, _ := wgtypes.GenerateKey()
	return &models.WireGuardPeer{
		PrivateKey:   priv.String(),
		PublicKey:    priv.PublicKey().String(),
		PresharedKey: psk.String(),
		AddressCIDR:  addressCIDR,
		ServerPub:    serverPub,
		Endpoint:     endpoint,
		AllowedIPs:   strings.Join(allowed, ","),
		Keepalive:    keepalive,
	}, nil
}
