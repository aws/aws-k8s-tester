package pipeline

import (
	"fmt"

	"github.com/buildkite/go-pipeline/internal/env"
	"github.com/buildkite/go-pipeline/ordered"
	"github.com/buildkite/interpolate"
)

// Pipeline models a pipeline.
//
// Standard caveats apply - see the package comment.
type Pipeline struct {
	Steps Steps          `yaml:"steps"`
	Env   *ordered.MapSS `yaml:"env,omitempty"`

	// RemainingFields stores any other top-level mapping items so they at least
	// survive an unmarshal-marshal round-trip.
	RemainingFields map[string]any `yaml:",inline"`
}

// MarshalJSON marshals a pipeline to JSON. Special handling is needed because
// yaml.v3 has "inline" but encoding/json has no concept of it.
func (p *Pipeline) MarshalJSON() ([]byte, error) {
	return inlineFriendlyMarshalJSON(p)
}

// UnmarshalOrdered unmarshals the pipeline from either []any (a legacy
// sequence of steps) or *ordered.MapSA (a modern pipeline configuration).
func (p *Pipeline) UnmarshalOrdered(o any) error {
	switch o := o.(type) {
	case *ordered.MapSA:
		// A pipeline can be a mapping.
		// Wrap in a secret type to avoid infinite recursion between this method
		// and ordered.Unmarshal.
		type wrappedPipeline Pipeline
		if err := ordered.Unmarshal(o, (*wrappedPipeline)(p)); err != nil {
			return fmt.Errorf("unmarshaling Pipeline: %w", err)
		}

	case []any:
		// A pipeline can be a sequence of steps.
		if err := ordered.Unmarshal(o, &p.Steps); err != nil {
			return fmt.Errorf("unmarshaling steps: %w", err)
		}

	default:
		return fmt.Errorf("unmarshaling Pipeline: unsupported type %T, want either *ordered.Map[string, any] or []any", o)
	}

	// Ensure Steps is never nil. Server side expects a sequence.
	if p.Steps == nil {
		p.Steps = Steps{}
	}
	return nil
}

// InterpolationEnv contains environment variables that may be interpolated into
// a pipeline. Users may define an equivalence between environment variable name, for example
// the environment variable names may case-insensitive.
type InterpolationEnv interface {
	Get(name string) (string, bool)
	Set(name string, value string)
}

// Interpolate interpolates variables defined in both interpolationEnv and p.Env into the pipeline.
// More specifically, it does these things:
//   - Interpolate pipeline.Env and copy the results into interpolationEnv, provided they don't
//     conflict, to apply later.
//   - Interpolate any string value in the rest of the pipeline.
func (p *Pipeline) Interpolate(interpolationEnv InterpolationEnv) error {
	if interpolationEnv == nil {
		interpolationEnv = env.New()
	}

	// Preprocess any env that are defined in the top level block and place them
	// into env for later interpolation into the rest of the pipeline.
	if err := p.interpolateEnvBlock(interpolationEnv); err != nil {
		return err
	}

	tf := envInterpolator{env: interpolationEnv}

	// Recursively go through the rest of the pipeline and perform environment
	// variable interpolation on strings. Interpolation is performed in-place.
	if err := interpolateSlice(tf, p.Steps); err != nil {
		return err
	}

	return interpolateMap(tf, p.RemainingFields)
}

// interpolateEnvBlock runs interpolate.Interpolate on each pair in p.Env,
// interpolating with the variables defined in interpolationEnv, and then adding the
// results back into p.Env. Since each environment variable in p.Env can
// be interpolated into later environment variables, we also add the results
// to interpolationEnv, but only if interpolationEnv does not already contain that variable
// as interpolationEnv has precedence over p.Env. For clarification, this means that
// if a variable name is interpolated to collide with a variable in the
// interpolationEnv, the interpolationEnv will take precedence.
func (p *Pipeline) interpolateEnvBlock(interpolationEnv InterpolationEnv) error {
	return p.Env.Range(func(k, v string) error {
		// We interpolate both keys and values.
		intk, err := interpolate.Interpolate(interpolationEnv, k)
		if err != nil {
			return err
		}

		// v is always a string in this case.
		intv, err := interpolate.Interpolate(interpolationEnv, v)
		if err != nil {
			return err
		}

		p.Env.Replace(k, intk, intv)

		// put it into the interpolationEnv for interpolation on a later iteration, but only if it is not
		// already there, as interpolationEnv has precedence over p.Env
		if _, exists := interpolationEnv.Get(intk); !exists {
			interpolationEnv.Set(intk, intv)
		}

		return nil
	})
}
