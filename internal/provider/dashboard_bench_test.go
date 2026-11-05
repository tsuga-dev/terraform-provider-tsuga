package provider

import (
	"context"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func BenchmarkDashboardPlan(b *testing.B) {
	ctx := context.Background()
	server := providerserver.NewProtocol6(New("test", "test", "test")())()

	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		b.Fatal(err)
	}
	objectType := schemaResp.ResourceSchemas["tsuga_dashboard"].ValueType().(tftypes.Object)

	graphsListType := objectType.AttributeTypes["graphs"].(tftypes.List)
	graphObjType := graphsListType.ElementType.(tftypes.Object)
	layoutObjType := graphObjType.AttributeTypes["layout"].(tftypes.Object)
	visObjType := graphObjType.AttributeTypes["visualization"].(tftypes.Object)
	noteObjType := visObjType.AttributeTypes["note"].(tftypes.Object)

	graphs := make([]tftypes.Value, 50)
	for i := range graphs {
		note := nullObjectValue(noteObjType, map[string]tftypes.Value{
			"note": tftypes.NewValue(tftypes.String, "hello"),
		})
		layout := nullObjectValue(layoutObjType, map[string]tftypes.Value{
			"x": tftypes.NewValue(tftypes.Number, 0),
			"y": tftypes.NewValue(tftypes.Number, i),
			"w": tftypes.NewValue(tftypes.Number, 4),
			"h": tftypes.NewValue(tftypes.Number, 2),
		})
		graphs[i] = nullObjectValue(graphObjType, map[string]tftypes.Value{
			"id":            tftypes.NewValue(tftypes.String, "g"+strconv.Itoa(i)),
			"layout":        layout,
			"visualization": nullObjectValue(visObjType, map[string]tftypes.Value{"note": note}),
		})
	}
	config := nullObjectValue(objectType, map[string]tftypes.Value{
		"name":   tftypes.NewValue(tftypes.String, "dash"),
		"owner":  tftypes.NewValue(tftypes.String, "team-1"),
		"graphs": tftypes.NewValue(graphsListType, graphs),
	})
	dv, err := tfprotov6.NewDynamicValue(objectType, config)
	if err != nil {
		b.Fatal(err)
	}
	req := &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "tsuga_dashboard",
		PriorState:       &dv,
		ProposedNewState: &dv,
		Config:           &dv,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		resp, err := server.PlanResourceChange(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
		if len(resp.Diagnostics) > 0 {
			b.Fatal(resp.Diagnostics[0].Summary, resp.Diagnostics[0].Detail)
		}
	}
}
