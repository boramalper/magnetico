package dht

import (
	"net"
	"time"

	"go.uber.org/zap"

	"github.com/boramalper/magnetico/cmd/magneticod/dht/mainline"
)

type TrawlingManager struct {
	// private
	output           chan Result
	trawlingServices []*mainline.TrawlingService
	indexingServices []*mainline.IndexingService
}

type Result interface {
	InfoHash() [20]byte
	PeerAddr() *net.TCPAddr
}

func NewTrawlingManager(tsAddrs []string, isAddrs []string, interval time.Duration) *TrawlingManager {
	manager := new(TrawlingManager)
	manager.output = make(chan Result, 20)

	// Trawling Services
	for _, addr := range tsAddrs {
		service := mainline.NewTrawlingService(
			addr,
			2000,
			interval,
			mainline.TrawlingServiceEventHandlers{
				OnResult: manager.onTrawlingResult,
			},
		)
		manager.trawlingServices = append(manager.trawlingServices, service)
		service.Start()
	}

	// Indexing Services
	for _, addr := range isAddrs {
		service := mainline.NewIndexingService(addr, 2 * time.Second, mainline.IndexingServiceEventHandlers{
			OnResult: manager.onIndexingResult,
		})
		manager.indexingServices = append(manager.indexingServices, service)
		service.Start()
	}

	return manager
}

func (m *TrawlingManager) onTrawlingResult(res mainline.TrawlingResult) {
	select {
	case m.output <- res:
	default:
		// TODO: should be a warn
		zap.L().Debug("DHT manager output ch is full, result dropped!")
	}
}

func (m *TrawlingManager) onIndexingResult(res mainline.IndexingResult) {
	select {
	case m.output <- res:
	default:
		// TODO: should be a warn
		zap.L().Debug("DHT manager output ch is full, idx result dropped!")
	}
}

func (m *TrawlingManager) Output() <-chan Result {
	return m.output
}

func (m *TrawlingManager) Terminate() {
	for _, service := range m.trawlingServices {
		service.Terminate()
	}
}
