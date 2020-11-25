package netlink

import (
	"github.com/vishvananda/netlink"
)

const (
	LinkType = "wireguard"
)

type Link struct {
	netlink.LinkAttrs
}

func (w *Link) Attrs() *netlink.LinkAttrs {
	return &w.LinkAttrs
}

func (w *Link) Type() string {
	return LinkType
}
