package dht

import (
	"github.com/boramalper/magnetico/cmd/magneticod/dht/mainline"
	"net"
)

type TrawlingManager struct {
	// private
	output   chan mainline.TrawlingResult
	services []*mainline.TrawlingService
}

func NewTrawlingManager(mlAddrs []*net.UDPAddr) *TrawlingManager {
	manager := new(TrawlingManager)
	manager.output = make(chan mainline.TrawlingResult)

	for _, addr := range mlAddrs {
		manager.services = append(manager.services, mainline.NewTrawlingService(
			addr,
			mainline.TrawlingServiceEventHandlers{
				OnResult: manager.onResult,
			},
		))
	}

	for _, service := range manager.services {
		service.Start()
	}

	return manager
}

func (m *TrawlingManager) onResult(res mainline.TrawlingResult) {
	m.output <- res
}

func (m *TrawlingManager) Output() <-chan mainline.TrawlingResult {
	return m.output
}

func (m *TrawlingManager) Terminate() {
	for _, service := range m.services {
		service.Terminate()
	}
}
