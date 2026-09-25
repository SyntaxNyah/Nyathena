// Copyright (C) 2026 SyntaxNyah
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Generic operator-defined text transforms (PunishmentCustomText). This is the
// engine behind the custom-command builder's "text" action: the transform rules
// are serialized to JSON and stored in PunishmentState.customData, then applied
// to every IC message the punished player sends. Because the rules are data
// rather than code, operators can invent new effects (like /shoe — periods to
// commas, sometimes the whole message becomes "tuff") without touching Go.
package athena

import (
	"encoding/json"
	"math/rand"
	"strings"
)

// customTextSpec is the JSON shape stored in PunishmentState.customData for a
// PunishmentCustomText punishment. Replace is an ordered find/replace list;
// Random, when non-empty, replaces the whole message with a random entry with
// probability Chance.
type customTextSpec struct {
	Replace []customTextReplace `json:"replace"`
	Random  []string            `json:"random"`
	Chance  float64             `json:"chance"`
}

type customTextReplace struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// defaultCustomTextChance is the probability of a whole-message random
// replacement when Random is set but Chance is omitted (or <= 0).
const defaultCustomTextChance = 0.25

// parseCustomTextSpec decodes a customData payload into a customTextSpec. An
// empty or unparsable payload yields a spec with no rules (identity transform).
func parseCustomTextSpec(customData string) customTextSpec {
	var spec customTextSpec
	if err := json.Unmarshal([]byte(customData), &spec); err != nil {
		return customTextSpec{}
	}
	return spec
}

// applyCustomText applies a custom text transform to one IC message. It is the
// live entry point; pick/roll are the global rand functions. Determinism is
// preserved for tests via applyCustomTextSpec, which takes pick/roll explicitly.
func applyCustomText(text, customData string) string {
	return applyCustomTextSpec(text, parseCustomTextSpec(customData), rand.Intn, rand.Float64)
}

// applyCustomTextSpec is the pure core of the transform: given a spec and
// injected randomness (pick returns an index in [0,n), roll returns a float in
// [0,1)), it applies the ordered find/replace rules and then, with probability
// Chance, swaps the whole message for a random entry from Random. Same spec +
// same pick/roll => same output.
func applyCustomTextSpec(text string, spec customTextSpec, pick func(int) int, roll func() float64) string {
	for _, r := range spec.Replace {
		text = strings.ReplaceAll(text, r.From, r.To)
	}
	if len(spec.Random) > 0 {
		chance := spec.Chance
		if chance <= 0 {
			chance = defaultCustomTextChance
		}
		if roll() < chance {
			return spec.Random[pick(len(spec.Random))]
		}
	}
	return text
}
