//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"
	"terraform-provider-tsuga/internal/provider"
)

func exportJSON(input string) (output string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			output = errorJSON(fmt.Sprintf("Terraform export failed: %v", recovered))
		}
	}()
	var request provider.TerraformExportRequest
	if err := json.Unmarshal([]byte(input), &request); err != nil {
		return errorJSON("invalid export request: " + err.Error())
	}
	result, err := provider.ExportTerraform(context.Background(), request)
	if err != nil {
		return errorJSON(err.Error())
	}
	encoded, err := json.Marshal(struct {
		OK     bool                           `json:"ok"`
		Result provider.TerraformExportResult `json:"result"`
	}{true, result})
	if err != nil {
		return errorJSON(err.Error())
	}
	return string(encoded)
}

func errorJSON(message string) string {
	encoded, _ := json.Marshal(struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}{false, message})
	return string(encoded)
}

func main() {
	done := make(chan struct{})
	export := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return errorJSON("expected one JSON string")
		}
		return exportJSON(args[0].String())
	})
	stop := js.FuncOf(func(_ js.Value, _ []js.Value) any { close(done); return nil })
	defer export.Release()
	defer stop.Release()
	js.Global().Set("tsugaTerraformExport", map[string]any{"exportJSON": export, "close": stop})
	<-done
}
