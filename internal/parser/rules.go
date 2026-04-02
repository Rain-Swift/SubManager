package parser

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"submanager/internal/domain"
)

type RuleParseResult struct {
	Document domain.RuleDocumentIR
	Warnings []string
}

type ruleYAML struct {
	Payload []string `yaml:"payload"`
	Rules   []string `yaml:"rules"`
}

func BuildRuleReferenceIR(sourceURL string) domain.RuleDocumentIR {
	return domain.RuleDocumentIR{
		Storage:   domain.RuleStorageReference,
		Format:    "reference",
		SourceURL: sourceURL,
		Reference: sourceURL,
		Metadata: map[string]string{
			"mode": "link_only",
		},
	}
}

func ParseRuleText(body []byte, sourceURL string) (RuleParseResult, error) {
	document := domain.RuleDocumentIR{
		Storage:   domain.RuleStorageInlineText,
		SourceURL: sourceURL,
	}

	lines, yamlMatched := parseRuleYAML(body)
	if yamlMatched {
		document.Format = "yaml"
	} else {
		document.Format = "plain_text"
		lines = splitNonEmptyLines(string(body))
	}

	entries, warnings := parseRuleEntries(lines)
	document.Entries = entries
	if len(entries) == 0 && strings.TrimSpace(string(body)) == "" {
		return RuleParseResult{}, fmt.Errorf("parse rules: empty body")
	}

	return RuleParseResult{
		Document: document,
		Warnings: warnings,
	}, nil
}

func parseRuleYAML(body []byte) ([]string, bool) {
	var cfg ruleYAML
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return nil, false
	}
	if len(cfg.Payload) > 0 {
		return cfg.Payload, true
	}
	if len(cfg.Rules) > 0 {
		return cfg.Rules, true
	}
	return nil, false
}

func parseRuleEntries(lines []string) ([]domain.RuleEntryIR, []string) {
	entries := make([]domain.RuleEntryIR, 0, len(lines))
	var warnings []string

	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := splitAndTrim(trimmed, ",")
		entry := domain.RuleEntryIR{
			Index: index,
			Raw:   trimmed,
		}

		if len(parts) == 0 {
			continue
		}
		if len(parts) == 1 {
			entry.Type = "RAW"
			entry.Value = parts[0]
			warnings = append(warnings, fmt.Sprintf("rule[%d]: unstructured line stored as RAW", index))
			entries = append(entries, entry)
			continue
		}

		entry.Type = parts[0]
		entry.Value = parts[1]
		if len(parts) > 2 {
			entry.Policy = parts[2]
		}
		if len(parts) > 3 {
			entry.Params = append([]string(nil), parts[3:]...)
		}
		entries = append(entries, entry)
	}

	return entries, warnings
}

func splitNonEmptyLines(text string) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func splitAndTrim(text, separator string) []string {
	rawParts := strings.Split(text, separator)
	out := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}
