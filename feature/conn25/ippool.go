// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

package conn25

import (
	"errors"
	"net/netip"

	"go4.org/netipx"
)

// errPoolExhausted is returned when there are no more addresses to iterate over.
var errPoolExhausted = errors.New("ip pool exhausted")

// addrQueue represents the collection of addresses returned to the pool.
// It is a queue because when we hand out returned addresses we want to
// release the oldest first.
type addrQueue []netip.Addr

func (q *addrQueue) returnAddr(v netip.Addr) {
	*q = append(*q, v)
}

func (q *addrQueue) nextAddr() (netip.Addr, error) {
	if len(*q) == 0 {
		return netip.Addr{}, errPoolExhausted
	}
	a := (*q)[0]
	*q = (*q)[1:]
	return a, nil
}

// ipSetIterator allows for iteration over all the addresses within a netipx.IPSet.
// netipx.IPSet has a Ranges call that returns the "minimum and sorted set of IP ranges that covers [the set]".
// netipx.IPRange is "an inclusive range of IP addresses from the same address family.". So we can iterate over
// all the addresses in the set by keeping a track of the last address we returned, calling Next on the last address
// to get the new one, and if we run off the edge of the current range, starting on the next one.
type ipSetIterator struct {
	// ranges defines the addresses in the pool
	ranges []netipx.IPRange
	// last is internal tracking of which the last address provided was.
	last netip.Addr
	// rangeIdx is internal tracking of which netipx.IPRange from the IPSet we are currently on.
	rangeIdx int
}

// next returns the next address from the set, or errPoolExhausted if we have
// iterated over the whole set.
func (ipsi *ipSetIterator) next() (netip.Addr, error) {
	if ipsi.rangeIdx >= len(ipsi.ranges) {
		// ipset is empty or we have iterated off the end
		return netip.Addr{}, errPoolExhausted
	}
	if !ipsi.last.IsValid() {
		// not initialized yet
		ipsi.last = ipsi.ranges[0].From()
		return ipsi.last, nil
	}
	currRange := ipsi.ranges[ipsi.rangeIdx]
	if ipsi.last == currRange.To() {
		// then we need to move to the next range
		ipsi.rangeIdx++
		if ipsi.rangeIdx >= len(ipsi.ranges) {
			return netip.Addr{}, errPoolExhausted
		}
		ipsi.last = ipsi.ranges[ipsi.rangeIdx].From()
		return ipsi.last, nil
	}
	ipsi.last = ipsi.last.Next()
	return ipsi.last, nil
}

func newIPPool(ipset *netipx.IPSet) *ippool {
	if ipset == nil {
		return &ippool{}
	}
	return &ippool{
		ipSetIterator: &ipSetIterator{ranges: ipset.Ranges()},
		returnedAddrs: []netip.Addr{},
	}
}

type ippool struct {
	ipSetIterator *ipSetIterator
	returnedAddrs addrQueue
}

func (ipp *ippool) next() (netip.Addr, error) {
	// first hand out all the addrs in the set in order
	if a, err := ipp.ipSetIterator.next(); err == nil {
		return a, nil
	}
	// then when they've all been handed out once, give them
	// out again in the order returned
	return ipp.returnedAddrs.nextAddr()
}

func (ipp *ippool) returnAddr(a netip.Addr) {
	ipp.returnedAddrs.returnAddr(a)
}
