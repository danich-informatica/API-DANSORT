package sorter

import (
	"API-GREENEX/internal/listeners"
	"context"
)

type Sorter struct {
	ID        int                      `json:"id"`
	Ubicacion string                   `json:"ubicacion"`
	Salidas   []Salida                 `json:"salidas"`
	Cognex    listeners.CognexListener `json:"cognex"`
	ctx       context.Context
	cancel    context.CancelFunc
}

func GetNewSorter(ID int, ubicacion string, salidas []Salida, cognex listeners.CognexListener) *Sorter {
	ctx, cancel := context.WithCancel(context.Background())
	return &Sorter{
		ID:        ID,
		Ubicacion: ubicacion,
		Salidas:   salidas,
		Cognex:    cognex,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (s *Sorter) Start() error {
	// Iniciar el listener de Cognex
	err := s.Cognex.Start()
	if err != nil {
		return err
	}

	return nil
}
