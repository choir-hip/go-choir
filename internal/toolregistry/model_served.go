package toolregistry

import (
	"encoding/json"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// emitModelServed writes the activation's model.served line: the models
// that answered its calls and every switch between them
// (docs/problems/model-provenance-unwired-2026-11-05.md). An activation
// that called no model and switched nothing writes nothing.
func emitModelServed(emit provideriface.EventEmitFunc, selected provideriface.LLMSelection, served *types.ModelServedTally) {
	if emit == nil || served.Empty() {
		return
	}
	rec := served.Record(types.ModelServedPurposeToolLoop)
	rec.SelectedProvider, rec.SelectedModel, rec.SelectedSource = selected.Provider, selected.Model, selected.Source
	payload, err := json.Marshal(rec)
	if err != nil {
		return
	}
	emit(types.EventModelServed, "", payload)
}

func modelSwitch(from, to provideriface.LLMSelection, reason string) types.ModelSwitch {
	return types.ModelSwitch{
		FromProvider: from.Provider, FromModel: from.Model,
		ToProvider: to.Provider, ToModel: to.Model,
		Reason: reason,
	}
}
