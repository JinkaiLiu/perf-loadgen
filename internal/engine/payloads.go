package engine

import (
	"os"
	"sync/atomic"

	"github.com/JinkaiLiu/vibeready/internal/config"
)

type payloadPicker struct {
	payloads [][]byte
	files    []string
	index    atomic.Uint64
}

func newPayloadPicker(cfg config.Config) *payloadPicker {
	if len(cfg.PayloadFiles) > 0 {
		return &payloadPicker{files: cfg.PayloadFiles}
	}
	if len(cfg.Payloads) > 0 {
		return &payloadPicker{payloads: cfg.Payloads}
	}
	if len(cfg.Body) > 0 {
		return &payloadPicker{payloads: [][]byte{cfg.Body}}
	}
	return &payloadPicker{}
}

func (p *payloadPicker) Next() []byte {
	if len(p.files) > 0 {
		index := p.index.Add(1) - 1
		path := p.files[index%uint64(len(p.files))]
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		return data
	}
	if len(p.payloads) == 0 {
		return nil
	}
	if len(p.payloads) == 1 {
		return p.payloads[0]
	}
	index := p.index.Add(1) - 1
	return p.payloads[index%uint64(len(p.payloads))]
}
