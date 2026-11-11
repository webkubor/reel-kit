package main

// Stored maps contain only JSON values: submission/import validates and normalizes
// them before insertion. Copy nested maps and arrays before leaving the store lock.
func cloneNestedMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = cloneNestedValue(v)
	}
	return dst
}

func cloneNestedValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return cloneNestedMap(x)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = cloneNestedValue(item)
		}
		return out
	case []string:
		return append([]string(nil), x...)
	default:
		return v
	}
}

func cloneAsset(src *Asset) *Asset {
	out := *src
	out.Extra = cloneNestedMap(src.Extra)
	if src.HasAudio != nil {
		v := *src.HasAudio
		out.HasAudio = &v
	}
	return &out
}

func cloneJob(src *Job) *Job {
	out := *src
	out.Input = cloneNestedMap(src.Input)
	out.Upstream = cloneNestedMap(src.Upstream)
	out.SOPSnapshot = cloneNestedMap(src.SOPSnapshot)
	out.Steps = append([]Step{}, src.Steps...)
	for i := range out.Steps {
		if v := out.Steps[i].StartedAt; v != nil {
			copy := *v
			out.Steps[i].StartedAt = &copy
		}
		if v := out.Steps[i].EndedAt; v != nil {
			copy := *v
			out.Steps[i].EndedAt = &copy
		}
	}
	if src.Approval != nil {
		v := *src.Approval
		out.Approval = &v
	}
	if src.Quality != nil {
		v := *src.Quality
		v.Checks = append([]QualityCheck(nil), src.Quality.Checks...)
		out.Quality = &v
	}
	return &out
}
