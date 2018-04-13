package dht

import "github.com/izolight/magnetico/cmd/magneticod/dht/mainline"

type TrawlingManager struct {
	// private
	output   chan mainline.TrawlingResult
	services []*mainline.TrawlingService
}

func NewTrawlingManager(mlAddrs []string) *TrawlingManager {
	manager := new(TrawlingManager)
	manager.output = make(chan mainline.TrawlingResult)

	if mlAddrs == nil {
		mlAddrs = []string{"0.0.0.0:0"}
	}
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
