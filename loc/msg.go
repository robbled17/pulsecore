// Package loc defines coded UI messages: backend sends Code+Params, frontend localizes.
package loc

// Msg is a machine-readable UI string. Never put human language in Code.
type Msg struct {
	Code   string                 `json:"code"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// M builds a Msg with optional params.
func M(code string, params map[string]interface{}) Msg {
	if params == nil {
		return Msg{Code: code}
	}
	return Msg{Code: code, Params: params}
}

// P is a tiny helper for one-off param maps.
func P(kv ...interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		out[key] = kv[i+1]
	}
	return out
}
