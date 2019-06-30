package wireguard_interface

import (
	"github.com/mrincompetent/wireguard-controller/pkg/wireguard"

	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

func (r *Reconciler) configureInterface(log *zap.Logger) error {
	link, err := netlink.LinkByName(r.interfaceName)
	if err != nil {
		if _, isNotFoundErr := err.(netlink.LinkNotFoundError); !isNotFoundErr {
			return errors.WithMessagef(err, "unable to get the interface %s", r.interfaceName)
		}
		log.Info("WireGuard interface does not exist. Creating...")

		// Create the interface as it does not exist
		link = &wireguard.Link{
			LinkAttrs: netlink.LinkAttrs{
				Name: r.interfaceName,
			},
		}
		if err := netlink.LinkAdd(link); err != nil {
			return errors.Wrap(err, "unable to create the interface")
		}
		log.Info("Created the WireGuard interface")
	}

	addresses, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return errors.Wrap(err, "unable to list interface addresses")
	}
	var found bool
	for _, existingAddr := range addresses {
		if existingAddr.Equal(*r.wgAddr) {
			found = true
			break
		}
	}
	if !found {
		if err := netlink.AddrAdd(link, r.wgAddr); err != nil {
			return errors.Wrap(err, "unable to set address on the interface")
		}
		log.Info("Configured address on WireGuard interface", zap.String("wireguard_address", r.wgAddr.String()))
	}

	if link.Attrs().OperState != netlink.OperUp {
		stateBefore := link.Attrs().OperState
		if err := netlink.LinkSetUp(link); err != nil {
			return errors.Wrap(err, "unable to bring up the interface")
		}

		// For some reason the interface state is unknown after bringing it up.
		// Thus we only log this message when the state was down before - otherwise we'll log this message on every sync
		if stateBefore == netlink.OperDown {
			log.Info("Brought WireGuard interface up", zap.String("state-before", stateBefore.String()))
		}
	}

	return nil
}
