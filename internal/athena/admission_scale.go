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

// Heat-scaled admission.
//
// The operator's connection/message/new-IP limits are tuned for an attack, but
// they used to apply at the same strength on a quiet evening. That is how a
// strict 5-connections-per-15-seconds limit turns a flaky WebSocket reconnect
// into a lockout, and how a 350-second new-IP OOC cooldown makes a normal
// newcomer think the server is broken.
//
// These helpers keep the configured limits as the UNDER-ATTACK values and let
// normal operation relax them. 100% means "use the configured value", which is
// the default and preserves the old behaviour exactly. The heat state is the
// existing raidGuardUnderAttack flag: correlation, arrival burst, or modcall
// fan-out must have marked the server under attack before the strict values are
// used.
package athena

// admissionLimit scales a count limit while the server is not under attack.
// It only ever relaxes, never tightens; a scale below 100 or a zero/negative
// base is treated as the configured value.
func admissionLimit(base int) int {
	if base <= 0 {
		return base
	}
	if raidGuardUnderAttack() {
		return base
	}
	scale := 100
	if config != nil {
		scale = config.AdmissionNormalScale
	}
	if scale <= 100 {
		return base
	}
	if scale > 500 {
		scale = 500
	}
	relaxed := base * scale / 100
	if relaxed < base {
		return base
	}
	return relaxed
}

// admissionNormalScale reports the configured normal-operation percentage,
// clamped to the same range admissionLimit uses. 100 means no relaxation.
func admissionNormalScale() int {
	if config == nil || config.AdmissionNormalScale <= 0 {
		return 100
	}
	if config.AdmissionNormalScale > 500 {
		return 500
	}
	return config.AdmissionNormalScale
}

// admissionCooldown returns the normal-operation cooldown when one is
// configured, otherwise the operator's (attack) value. A normal value larger
// than the configured one is clamped down: relaxing admission must never make a
// cooldown longer than the operator asked for.
func admissionCooldown(configured, normal int) int {
	if configured <= 0 {
		return configured
	}
	if raidGuardUnderAttack() || normal <= 0 || normal > configured {
		return configured
	}
	return normal
}
