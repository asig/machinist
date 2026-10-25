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
	"strconv"

	"github.com/asig/machinist/internal/expr"
	"github.com/asig/machinist/internal/model"
)

// evalLayer expands ${...} expressions in all string values of l in place.
// "self" refers to the raw values of l, "parent" to the machine merged from
// all ancestors so far.
func evalLayer(l *layer, parent model.Machine) error {
	env := map[string]any{
		"self":   l.Raw,
		"parent": machineView(parent),
	}

	var err error
	if l.Binary != nil {
		var binary string
		if binary, err = evalString(l, env, "binary", *l.Binary); err != nil {
			return err
		}
		if binary == "" {
			return l.valueError("binary", "must not be empty")
		}
		l.Binary = &binary
	}
	for _, key := range scalarKeys {
		value, ok := l.Scalars[key]
		if !ok {
			continue
		}
		if l.Scalars[key], err = evalString(l, env, key, value); err != nil {
			return err
		}
	}
	for id, props := range l.Netdevs {
		if err := evalProps(l, env, "netdev."+id, "type", props); err != nil {
			return err
		}
	}
	for id, props := range l.Audiodevs {
		if err := evalProps(l, env, "audiodev."+id, "driver", props); err != nil {
			return err
		}
	}
	for i, props := range l.Devices {
		if err := evalProps(l, env, fmt.Sprintf("device[%d]", i), "driver", props); err != nil {
			return err
		}
	}
	for i, props := range l.Drives {
		if err := evalProps(l, env, fmt.Sprintf("drive[%d]", i), "", props); err != nil {
			return err
		}
	}

	if l.ExtraArgsExpr != nil {
		value, err := expr.Eval(*l.ExtraArgsExpr, env)
		if err != nil {
			return l.valueError("extra_args", err.Error())
		}
		list, ok := value.([]any)
		if !ok {
			return l.valueError("extra_args", "expression must yield a list")
		}
		if l.ExtraArgs, err = stringList(l, "extra_args", list); err != nil {
			return err
		}
	} else if l.ExtraArgs != nil {
		if l.ExtraArgs, err = evalStringList(l, env, "extra_args", l.ExtraArgs); err != nil {
			return err
		}
	}
	return nil
}

func evalString(l *layer, env map[string]any, keyPath, text string) (string, error) {
	value, err := expr.Eval(text, env)
	if err != nil {
		return "", l.valueError(keyPath, err.Error())
	}
	result, err := expr.Stringify(value)
	if err != nil {
		return "", l.valueError(keyPath, err.Error())
	}
	return result, nil
}

// evalProps keeps the type of whole-string expressions, so "${1 + 2}" becomes
// an int64. The lead key must stay a string.
func evalProps(l *layer, env map[string]any, keyPath, lead string, props model.Props) error {
	for key, value := range props {
		text, ok := value.(string)
		if !ok {
			continue
		}
		result, err := expr.Eval(text, env)
		if err != nil {
			return l.valueError(keyPath+"."+key, err.Error())
		}
		switch result.(type) {
		case string, int64, bool:
			props[key] = result
		default:
			return l.valueError(keyPath+"."+key, "expression must yield a string, integer or boolean")
		}
	}
	if lead != "" {
		return requireStringProperty(l, keyPath, lead, props)
	}
	return nil
}

// evalStringList evaluates each item. An item that yields a list is spliced.
func evalStringList(l *layer, env map[string]any, keyPath string, items []string) ([]string, error) {
	out := make([]string, 0, len(items))
	for i, item := range items {
		path := fmt.Sprintf("%s[%d]", keyPath, i)
		value, err := expr.Eval(item, env)
		if err != nil {
			return nil, l.valueError(path, err.Error())
		}
		if list, ok := value.([]any); ok {
			spliced, err := stringList(l, path, list)
			if err != nil {
				return nil, err
			}
			out = append(out, spliced...)
			continue
		}
		text, err := expr.Stringify(value)
		if err != nil {
			return nil, l.valueError(path, err.Error())
		}
		out = append(out, text)
	}
	return out, nil
}

func stringList(l *layer, keyPath string, list []any) ([]string, error) {
	out := make([]string, len(list))
	for i, item := range list {
		text, err := expr.Stringify(item)
		if err != nil {
			return nil, l.valueError(fmt.Sprintf("%s[%d]", keyPath, i), err.Error())
		}
		out[i] = text
	}
	return out, nil
}

// machineView exposes a merged machine to expressions. Unset scalars are
// omitted and decimal scalars become integers so that arithmetic works.
func machineView(m model.Machine) map[string]any {
	view := map[string]any{}
	scalars := map[string]string{
		"binary": m.Binary, "machine": m.Machine, "cpu": m.CPU,
		"smp": m.SMP, "memory": m.Memory, "vga": m.VGA, "display": m.Display,
	}
	for key, value := range scalars {
		if value == "" {
			continue
		}
		if n, err := strconv.ParseInt(value, 10, 64); err == nil && strconv.FormatInt(n, 10) == value {
			view[key] = n
		} else {
			view[key] = value
		}
	}
	view["netdev"] = propsMapView(m.Netdevs)
	view["audiodev"] = propsMapView(m.Audiodevs)
	view["device"] = propsListView(m.Devices)
	view["drive"] = propsListView(m.Drives)
	args := make([]any, len(m.ExtraArgs))
	for i, arg := range m.ExtraArgs {
		args[i] = arg
	}
	view["extra_args"] = args
	return view
}

func propsMapView(m map[string]model.Props) map[string]any {
	out := make(map[string]any, len(m))
	for id, props := range m {
		out[id] = map[string]any(props)
	}
	return out
}

func propsListView(list []model.Props) []any {
	out := make([]any, len(list))
	for i, props := range list {
		out[i] = map[string]any(props)
	}
	return out
}
