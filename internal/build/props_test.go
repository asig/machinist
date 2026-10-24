/*
 * This file is part of Machinist.
 *
 * Copyright (C) 2026 Andreas Signer <asigner@gmail.com>
 *
 * Machinist is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License
 * as published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * Machinist is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with Machinist.  If not, see <https://www.gnu.org/licenses/>.
 */

package build

import (
	"strings"
	"testing"

	"github.com/asig/machinist/internal/model"
)

func TestFormatPropsOrderingAndEscaping(t *testing.T) {
	props := model.Props{
		"driver":  "virtio,net",
		"zeta":    "last,value",
		"alpha":   "first",
		"queues":  int64(2),
		"enabled": true,
	}

	got, err := formatProps("driver", "net0,primary", props)
	if err != nil {
		t.Fatalf("formatProps returned error: %v", err)
	}
	want := "virtio,,net,id=net0,,primary,alpha=first,enabled=on,queues=2,zeta=last,,value"
	if got != want {
		t.Errorf("formatProps = %q, want %q", got, want)
	}
}

func TestFormatPropsWithoutLeadingKey(t *testing.T) {
	props := model.Props{
		"file": "disk.qcow2",
		"bus":  int64(0),
		"if":   "ide",
		"id":   "disk0",
		"boot": false,
	}

	got, err := formatProps("", "", props)
	if err != nil {
		t.Fatalf("formatProps returned error: %v", err)
	}
	if want := "id=disk0,boot=off,bus=0,file=disk.qcow2,if=ide"; got != want {
		t.Errorf("formatProps = %q, want %q", got, want)
	}
}

func TestFormatPropsRequiresStringLeadingKey(t *testing.T) {
	tests := []struct {
		name  string
		props model.Props
		want  string
	}{
		{name: "missing", props: model.Props{"id": "net0"}, want: `required property "driver" is missing`},
		{name: "not string", props: model.Props{"driver": int64(1)}, want: `property "driver" must be a string`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := formatProps("driver", "", tt.props)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("formatProps error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value any
		want  string
	}{
		{value: "one,two", want: "one,,two"},
		{value: int64(-42), want: "-42"},
		{value: true, want: "on"},
		{value: false, want: "off"},
	}

	for _, tt := range tests {
		got, err := formatValue(tt.value)
		if err != nil {
			t.Errorf("formatValue(%#v) returned error: %v", tt.value, err)
			continue
		}
		if got != tt.want {
			t.Errorf("formatValue(%#v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestFormatValueRejectsUnsupportedTypes(t *testing.T) {
	for _, value := range []any{1.5, []string{"nested"}, map[string]any{"nested": true}} {
		if _, err := formatValue(value); err == nil {
			t.Errorf("formatValue(%T) returned nil error", value)
		}
	}
}

func TestFormatPropsReportsUnsupportedProperty(t *testing.T) {
	_, err := formatProps("", "", model.Props{"unsupported": 1.5})
	if err == nil || !strings.Contains(err.Error(), `property "unsupported"`) {
		t.Fatalf("formatProps error = %v, want unsupported property context", err)
	}
}
