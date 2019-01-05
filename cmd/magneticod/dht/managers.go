package dht

import (
	"net"
	"time"

	"go.uber.org/zap"

	"github.com/boramalper/magnetico/cmd/magneticod/dht/mainline"
)

type TrawlingManager struct {
	// private
	output   chan Result
	services []*mainline.TrawlingService
}

type Result interface {
	InfoHash() [20]byte
	PeerAddr() *net.TCPAddr
}

func NewTrawlingManager(mlAddrs []string, interval time.Duration) *TrawlingManager {
	manager := new(TrawlingManager)
	manager.output = make(chan Result, 20)

	if mlAddrs == nil {
		mlAddrs = []string{"0.0.0.0:0"}
	}
	for _, addr := range mlAddrs {
		manager.services = append(manager.services, mainline.NewTrawlingService(
			addr,
			2000,
			interval,
			mainline.TrawlingServiceEventHandlers{
				OnResult: manager.onTrawlingResult,
			},
		))
	}

	for _, service := range manager.services {
		service.Start()
	}

	return manager
}

func (m *TrawlingManager) onTrawlingResult(res mainline.TrawlingResult) {
	select {
	case m.output <- res:
	default:
		zap.L().Warn("DHT manager output ch is full, result dropped!")
	}
}

func (m *TrawlingManager) Output() <-chan Result {
	return m.output
}

func (m *TrawlingManager) Terminate() {
	for _, service := range m.services {
		service.Terminate()
	}
}
