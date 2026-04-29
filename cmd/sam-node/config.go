package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/biscuit-auth/biscuit-go/v2"
	"github.com/biscuit-auth/biscuit-go/v2/parser"
	"github.com/google/sam/api"
	"gopkg.in/yaml.v2"
)

type CompiledLocalPolicy struct {
	Policies []biscuit.Policy
	Checks   []biscuit.Check
	Rules    []biscuit.Rule
}

// LoadLocalPolicy loads the local policy configuration from the specified path.
// If the file is missing, it returns an empty initialized config.
func LoadLocalPolicy(path string) (*CompiledLocalPolicy, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &CompiledLocalPolicy{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config api.LocalPolicy
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	compiled := &CompiledLocalPolicy{}

	for _, pStr := range config.Attenuation.Policies {
		trimmed := strings.TrimRight(strings.TrimSpace(pStr), ";")
		p, err := parser.FromStringPolicy(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid local policy syntax %q: %w", pStr, err)
		}
		compiled.Policies = append(compiled.Policies, p)
	}

	for _, cStr := range config.Attenuation.Checks {
		trimmed := strings.TrimRight(strings.TrimSpace(cStr), ";")
		c, err := parser.FromStringCheck(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid local check syntax %q: %w", cStr, err)
		}
		compiled.Checks = append(compiled.Checks, c)
	}

	for _, rStr := range config.Attenuation.Rules {
		trimmed := strings.TrimRight(strings.TrimSpace(rStr), ";")
		r, err := parser.FromStringRule(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid local rule syntax %q: %w", rStr, err)
		}
		compiled.Rules = append(compiled.Rules, r)
	}

	return compiled, nil
}
