package mc

import (
	"context"
	"errors"
	"fmt"
	p2p_host "github.com/libp2p/go-libp2p-host"
	p2p_metrics "github.com/libp2p/go-libp2p-metrics"
	p2p_peer "github.com/libp2p/go-libp2p-peer"
	p2p_pstore "github.com/libp2p/go-libp2p-peerstore"
	p2p_swarm "github.com/libp2p/go-libp2p-swarm"
	p2p_bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	multiaddr "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
	"log"
	"net"
	"strings"
)

func Hash(data []byte) multihash.Multihash {
	// this can only fail with an error because of an incorrect hash family
	mh, _ := multihash.Sum(data, multihash.SHA2_256, -1)
	return mh
}

func HashFromBytes(hash []byte) multihash.Multihash {
	mhash := make([]byte, len(hash)+2)
	mhash[0] = multihash.SHA2_256
	mhash[1] = 32
	copy(mhash[2:], hash)
	return mhash
}

func ParseAddress(str string) (multiaddr.Multiaddr, error) {
	return multiaddr.NewMultiaddr(str)
}

var BadHandle = errors.New("Bad handle")

// handle: multiaddr/p2p/id
func FormatHandle(pi p2p_pstore.PeerInfo) string {
	if len(pi.Addrs) > 0 {
		return fmt.Sprintf("%s/p2p/%s", pi.Addrs[0].String(), pi.ID.Pretty())
	} else {
		return fmt.Sprintf("/p2p/%s", pi.ID.Pretty())
	}
}

// parses handles into multiaddrs
// Canonical forms:
//  /multiaddr/p2p/id
//  /p2p/id
// Also accepts for backwards compatibility:
//  /multiaddr/id
//  id
func ParseHandle(str string) (empty p2p_pstore.PeerInfo, err error) {
	var id, addr string

	ix := strings.LastIndex(str, "/")
	if ix < 0 {
		return ParseHandleId(str)
	}
	id = str[ix+1:]

	iy := strings.LastIndex(str[:ix], "/")
	switch {
	case iy < 0:
		addr = str[:ix]
	case str[iy+1:ix] == "p2p":
		addr = str[:iy]
	default:
		addr = str[:ix]
	}

	pid, err := p2p_peer.IDB58Decode(id)
	if err != nil {
		return empty, err
	}

	if addr == "" {
		return p2p_pstore.PeerInfo{ID: pid}, nil
	}

	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return empty, err
	}

	return p2p_pstore.PeerInfo{ID: pid, Addrs: []multiaddr.Multiaddr{maddr}}, nil
}

func ParseHandleId(str string) (empty p2p_pstore.PeerInfo, err error) {
	pid, err := p2p_peer.IDB58Decode(str)
	if err != nil {
		return empty, err
	}

	return p2p_pstore.PeerInfo{ID: pid}, nil
}

// re-export this option to avoid basic host interface leakage
const NATPortMap = p2p_bhost.NATPortMap

func NewHost(ctx context.Context, id PeerIdentity, laddr multiaddr.Multiaddr, opts ...interface{}) (p2p_host.Host, error) {
	pstore := p2p_pstore.NewPeerstore()
	pstore.AddPrivKey(id.ID, id.PrivKey)
	pstore.AddPubKey(id.ID, id.PrivKey.GetPublic())

	var addrs []multiaddr.Multiaddr
	if laddr != nil {
		addrs = []multiaddr.Multiaddr{laddr}
	}

	netw, err := p2p_swarm.NewNetwork(
		context.Background(),
		addrs,
		id.ID,
		pstore,
		p2p_metrics.NewBandwidthCounter())
	if err != nil {
		return nil, err
	}

	return p2p_bhost.New(netw, opts...), nil
}

// multiaddr juggling
func isAddrSubnet(addr multiaddr.Multiaddr, nets []*net.IPNet) bool {
	ipstr, err := addr.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		return false
	}

	ip := net.ParseIP(ipstr)
	if ip == nil {
		return false
	}

	for _, net := range nets {
		if net.Contains(ip) {
			return true
		}
	}

	return false
}

var (
	localhostCIDR  = []string{"127.0.0.0/8"}
	linkLocalCIDR  = []string{"169.254.0.0/16"}
	privateCIDR    = []string{"10.0.0.0/8", "100.64.0.0/10", "172.16.0.0/12", "192.168.0.0/16"}
	unroutableCIDR = []string{"0.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16"}
	internalCIDR   = []string{"0.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16", "10.0.0.0/8", "100.64.0.0/10", "172.16.0.0/12", "192.168.0.0/16"}
)

var (
	localhostSubnet  []*net.IPNet
	linkLocalSubnet  []*net.IPNet
	privateSubnet    []*net.IPNet
	unroutableSubnet []*net.IPNet
	internalSubnet   []*net.IPNet
)

func makeSubnetSpec(subnets []string) []*net.IPNet {
	nets := make([]*net.IPNet, len(subnets))
	for x, subnet := range subnets {
		_, net, err := net.ParseCIDR(subnet)
		if err != nil {
			log.Fatal(err)
		}
		nets[x] = net
	}
	return nets
}

func init() {
	localhostSubnet = makeSubnetSpec(localhostCIDR)
	linkLocalSubnet = makeSubnetSpec(linkLocalCIDR)
	privateSubnet = makeSubnetSpec(privateCIDR)
	unroutableSubnet = makeSubnetSpec(unroutableCIDR)
	internalSubnet = makeSubnetSpec(internalCIDR)
}

func IsLocalhostAddr(addr multiaddr.Multiaddr) bool {
	return isAddrSubnet(addr, localhostSubnet)
}

func IsLinkLocalAddr(addr multiaddr.Multiaddr) bool {
	return isAddrSubnet(addr, linkLocalSubnet)
}

func IsPrivateAddr(addr multiaddr.Multiaddr) bool {
	return isAddrSubnet(addr, privateSubnet)
}

func IsRoutableAddr(addr multiaddr.Multiaddr) bool {
	return !isAddrSubnet(addr, unroutableSubnet)
}

func IsPublicAddr(addr multiaddr.Multiaddr) bool {
	return !isAddrSubnet(addr, internalSubnet)
}

func FilterAddrs(addrs []multiaddr.Multiaddr, predf func(multiaddr.Multiaddr) bool) []multiaddr.Multiaddr {
	res := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		if predf(addr) {
			res = append(res, addr)
		}
	}
	return res
}
