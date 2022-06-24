package rtc

import (
	"errors"
	"sync"
)

type PortsAllocator struct {
	sync.Mutex
	udpPorts map[int]bool
}

func NewPortsAllocator(rangeStart, rangeEnd int) *PortsAllocator {
	p := &PortsAllocator{
		udpPorts: make(map[int]bool),
	}

	for i := rangeStart; i < rangeEnd; i++ {
		p.udpPorts[i] = false
	}

	return p
}

func (p *PortsAllocator) Allocate() (int, error) {
	p.Lock()
	defer p.Unlock()

	for port, allocated := range p.udpPorts {
		if !allocated {
			p.udpPorts[port] = true
			return port, nil
		}
	}

	return 0, errors.New("no free ports")
}

func (p *PortsAllocator) Deallocate(port int) {
	p.Lock()
	p.udpPorts[port] = false
	p.Unlock()
}
