package quality

import "slices"

// Clones copy mutable slices and pointers without serializing large model outputs.
func cloneValue[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneConfig(config RunConfig) RunConfig {
	config.Models = slices.Clone(config.Models)
	config.Parameters.Temperature = cloneValue(config.Parameters.Temperature)
	return config
}

func cloneFrozen(frozen Frozen) Frozen {
	frozen.RunConfig = cloneConfig(frozen.RunConfig)
	frozen.ManagedProbe = cloneValue(frozen.ManagedProbe)
	return frozen
}

func cloneSamples(samples []Sample) []Sample {
	out := slices.Clone(samples)
	for i := range out {
		out[i].TTFTMS = cloneValue(out[i].TTFTMS)
		out[i].Usage = cloneValue(out[i].Usage)
		out[i].JudgeUsage = cloneValue(out[i].JudgeUsage)
	}
	return out
}

func cloneRun(run *Run) *Run {
	if run == nil {
		return nil
	}
	out := *run
	out.Config = cloneFrozen(run.Config)
	out.CompletedAt = cloneValue(run.CompletedAt)
	out.Samples = cloneSamples(run.Samples)
	out.Controls = cloneSamples(run.Controls)
	return &out
}

func cloneRule(rule *Rule) *Rule {
	if rule == nil {
		return nil
	}
	out := *rule
	out.RunConfig = cloneConfig(rule.RunConfig)
	out.LastManualSkips = slices.Clone(rule.LastManualSkips)
	return &out
}
