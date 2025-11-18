package values

var ContourValues = Override{
	"gatewayAPI": map[string]any{
		"manageCRDs": false,
	},
	"configInline": map[string]any{
		"gateway": map[string]any{
			"gatewayRef": map[string]any{
				"name":      "korifi",
				"namespace": "cfapi-system",
			},
		},
	},
}
