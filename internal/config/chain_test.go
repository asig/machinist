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

package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestResolveChainRootFirst(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "base", "machine = \"base\"\n")
	writeConfig(t, root, "dos/common", "parent = \"base\"\ncpu = \"common\"\n")
	writeConfig(t, root, "dos/win311", "parent = \"dos/common\"\nmemory = \"16M\"\n")

	layers, err := resolveChain(root, "dos/win311")
	if err != nil {
		t.Fatalf("resolveChain returned error: %v", err)
	}
	if got, want := layerNames(layers), []string{"base", "dos/common", "dos/win311"}; !equalStrings(got, want) {
		t.Errorf("chain = %#v, want %#v", got, want)
	}
}

func TestResolveChainParentIsRelativeToRoot(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "common/base", "cpu = \"root-base\"\n")
	writeConfig(t, root, "dos/common/base", "cpu = \"wrong-base\"\n")
	writeConfig(t, root, "dos/win311", "parent = \"common/base\"\n")

	layers, err := resolveChain(root, "dos/win311")
	if err != nil {
		t.Fatalf("resolveChain returned error: %v", err)
	}
	if got, want := layerNames(layers), []string{"common/base", "dos/win311"}; !equalStrings(got, want) {
		t.Errorf("chain = %#v, want %#v", got, want)
	}
	if got := layers[0].Scalars["cpu"]; got != "root-base" {
		t.Errorf("root parent cpu = %q, want root-base", got)
	}
}

func TestResolveChainReportsMissingParentAndChild(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "dos/win311", "parent = \"common/base\"\n")

	_, err := resolveChain(root, "dos/win311")
	if err == nil {
		t.Fatal("resolveChain returned nil error, want missing-parent error")
	}
	for _, expected := range []string{"dos/win311", "common/base", "common/base.toml"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("error %q does not contain %q", err, expected)
		}
	}
}

func TestResolveChainReportsCycle(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "dos/win311", "parent = \"dos/common\"\n")
	writeConfig(t, root, "dos/common", "parent = \"tools/base\"\n")
	writeConfig(t, root, "tools/base", "parent = \"dos/common\"\n")

	_, err := resolveChain(root, "dos/win311")
	if err == nil {
		t.Fatal("resolveChain returned nil error, want cycle error")
	}
	if want := "dos/common -> tools/base -> dos/common"; !strings.Contains(err.Error(), want) {
		t.Fatalf("resolveChain error = %q, want cycle %q", err, want)
	}
}

func TestResolveChainEnforcesMaximumDepth(t *testing.T) {
	for _, depth := range []int{maxChainDepth, maxChainDepth + 1} {
		t.Run(fmt.Sprintf("depth_%d", depth), func(t *testing.T) {
			root := t.TempDir()
			for i := 0; i < depth; i++ {
				contents := ""
				if i+1 < depth {
					contents = fmt.Sprintf("parent = \"config-%d\"\n", i+1)
				}
				writeConfig(t, root, fmt.Sprintf("config-%d", i), contents)
			}

			layers, err := resolveChain(root, "config-0")
			if depth == maxChainDepth {
				if err != nil {
					t.Fatalf("resolveChain at limit returned error: %v", err)
				}
				if len(layers) != maxChainDepth {
					t.Fatalf("got %d layers, want %d", len(layers), maxChainDepth)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "maximum depth") {
				t.Fatalf("resolveChain beyond limit error = %v, want depth error", err)
			}
		})
	}
}

func TestResolveChainValidatesRequestedName(t *testing.T) {
	_, err := resolveChain(t.TempDir(), "../outside")
	if err == nil || !strings.Contains(err.Error(), "dot segments") {
		t.Fatalf("resolveChain invalid name error = %v", err)
	}
}

func layerNames(layers []*layer) []string {
	names := make([]string, len(layers))
	for i, current := range layers {
		names[i] = current.Name
	}
	return names
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
