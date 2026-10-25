/*
 * This file is part of Machinist.
 *
 * Copyright (C) 2026 Andreas Signer <asigner@gmail.com>
 *
 * Machinist is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Machinist is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with Machinist.  If not, see <https://www.gnu.org/licenses/>.
 */

package expr

import (
	"reflect"
	"strings"
	"testing"
)

func TestEval(t *testing.T) {
	env := map[string]any{
		"parent": map[string]any{
			"memory": int64(1024),
			"cpu":    "host",
			"args":   []any{"-S"},
			"device": []any{map[string]any{"driver": "e1000"}},
			"netdev": map[string]any{"net-0": map[string]any{"type": "user"}},
		},
	}
	tests := []struct {
		in   string
		want any
	}{
		{"plain", "plain"},
		{"${1 + 2 * 3}", int64(7)},
		{"${(1 + 2) * 3}", int64(9)},
		{"${10 - 4 - 3}", int64(3)},
		{"${7 / 2}", int64(3)},
		{"${7 % 4}", int64(3)},
		{"${-parent.memory}", int64(-1024)},
		{"${parent.memory + 512}", int64(1536)},
		{"${parent.memory * 2}M", "2048M"},
		{"cpu-${parent.cpu}", "cpu-host"},
		{"${parent.cpu + '-x'}", "host-x"},
		{"${\"a}b\" + 'c'}", "a}bc"},
		{"${true}", true},
		{"flag=${true}", "flag=on"},
		{"${'x' + false}", "xoff"},
		{"${[]}", []any{}},
		{"${parent.args + ['-s']}", []any{"-S", "-s"}},
		{"${parent.args + '-s'}", []any{"-S", "-s"}},
		{"${parent.args + [1, 2 + 3]}", []any{"-S", int64(1), int64(5)}},
		{"${parent.device[0].driver}", "e1000"},
		{"${parent.netdev['net-0'].type}", "user"},
		{"$${literal}", "${literal}"},
		{"${ 'x' }", "x"},
	}
	for _, tt := range tests {
		got, err := Eval(tt.in, env)
		if err != nil {
			t.Errorf("Eval(%q): %v", tt.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Eval(%q) = %#v, want %#v", tt.in, got, tt.want)
		}
	}
}

func TestEvalErrors(t *testing.T) {
	env := map[string]any{"parent": map[string]any{"args": []any{"a"}, "n": int64(1)}}
	tests := []struct{ in, want string }{
		{"${", "unexpected end"},
		{"${1 +", "unexpected end"},
		{"${1", `expected "}"`},
		{"${unknown}", "unknown variable"},
		{"${parent.missing}", "unknown property"},
		{"${parent.n.x}", "cannot read property"},
		{"${1 / 0}", "division by zero"},
		{"${1 % 0}", "division by zero"},
		{"${'a' - 1}", "needs integers"},
		{"${parent.args[5]}", "out of range"},
		{"${parent.args['x']}", "list index must be an integer"},
		{"${'abc}", "unterminated string"},
		{"x${parent.args}", "cannot use list as a string"},
		{"${parent + 1}", "cannot use object as a string"},
		{"${[1 2]}", `expected ","`},
		{"${1 $ 2}", `expected "}"`},
	}
	for _, tt := range tests {
		_, err := Eval(tt.in, env)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Eval(%q) error = %v, want containing %q", tt.in, err, tt.want)
		}
	}
}
