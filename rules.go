package main

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// Property values
const (
	PropertyFrom          = "from"
	PropertyTo            = "to"
	PropertySubject       = "subject"
	PropertyHas           = "hasTheWord"
	PropertyDoesNotHave   = "doesNotHaveTheWord"
	PropertyMarkImportant = "shouldAlwaysMarkAsImportant"
	PropertyApplyLabel    = "label"
	PropertyApplyCategory = "smartLabelToApply"
	PropertyDelete        = "shouldTrash"
	PropertyArchive       = "shouldArchive"
	PropertyMarkRead      = "shouldMarkAsRead"
)

// SmartLabel values
const (
	SmartLabelPersonal     = "personal"
	SmartLabelGroup        = "group"
	SmartLabelNotification = "notification"
	SmartLabelPromo        = "promo"
	SmartLabelSocial       = "social"
)

// Entry is a Gmail filter
type Entry []Property

// Property is a property of a Gmail filter, as specified by its XML format
type Property struct {
	Name  string
	Value string
}

// GenerateRules translates a config into entries that map directly into Gmail filters
func GenerateRules(config Config) ([]Entry, error) {
	res := []Entry{}
	for i, rule := range config.Rules {
		entries, err := generateRule(rule, config.Consts)
		if err != nil {
			return res, errors.Wrap(err, fmt.Sprintf("error generating rule #%d", i))
		}
		res = append(res, entries...)
	}
	return res, nil
}

func generateRule(rule Rule, consts Consts) ([]Entry, error) {
	filters, err := generateFilters(rule.Filters, consts)
	if err != nil {
		return nil, errors.Wrap(err, "error generating filters")
	}
	if len(filters) == 0 {
		return nil, errors.New("at least one filter has to be specified")
	}
	actions, err := generateActions(rule.Actions)
	if err != nil {
		return nil, errors.Wrap(err, "error generating actions")
	}
	if len(actions) == 0 {
		return nil, errors.New("at least one action has to be specified")
	}
	return combineFiltersActions(filters, actions), nil
}

func generateFilters(filters Filters, consts Consts) ([]Property, error) {
	res := []Property{}
	// simple filters first
	mf, err := generateMatchFilters(filters.CompositeFilters.MatchFilters)
	if err != nil {
		return nil, errors.Wrap(err, "error generating match filters")
	}
	res = append(res, mf...)

	// then simple filters with consts
	resolved, err := resolveFiltersConsts(filters.Consts.MatchFilters, consts)
	if err != nil {
		return nil, errors.Wrap(err, "error resolving consts in filter")
	}
	mf, err = generateMatchFilters(resolved)
	if err != nil {
		return nil, errors.Wrap(err, "error generating const match filters")
	}
	res = append(res, mf...)

	// TODO Not
	// The negation looks like:
	// -{to:{foobar@baz.com} } -{"Build failed"}
	// which are mapped to hasTheWord and doesNotHaveTheWord
	return res, nil
}

func resolveFiltersConsts(mf MatchFilters, consts Consts) (MatchFilters, error) {
	from, err := resolveConsts(mf.From, consts)
	if err != nil {
		return mf, errors.Wrap(err, "error in resolving 'from' clause")
	}
	to, err := resolveConsts(mf.To, consts)
	if err != nil {
		return mf, errors.Wrap(err, "error in resolving 'to' clause")
	}
	sub, err := resolveConsts(mf.Subject, consts)
	if err != nil {
		return mf, errors.Wrap(err, "error in resolving 'subject' clause")
	}
	has, err := resolveConsts(mf.Has, consts)
	if err != nil {
		return mf, errors.Wrap(err, "error in resolving 'has' clause")
	}
	res := MatchFilters{
		From:    from,
		To:      to,
		Subject: sub,
		Has:     has,
	}
	return res, nil
}

func resolveConsts(a []string, consts Consts) ([]string, error) {
	res := []string{}
	for _, s := range a {
		resolved, ok := consts[s]
		if !ok {
			return nil, fmt.Errorf("failed to resolve const '%s'", s)
		}
		res = append(res, resolved.Values...)
	}
	return res, nil
}

func generateMatchFilters(filters MatchFilters) ([]Property, error) {
	res := []Property{}
	if len(filters.From) > 0 {
		p := Property{PropertyFrom, joinOR(filters.From)}
		res = append(res, p)
	}
	if len(filters.To) > 0 {
		p := Property{PropertyTo, joinOR(filters.To)}
		res = append(res, p)
	}
	if len(filters.Subject) > 0 {
		p := Property{PropertySubject, joinOR(filters.Subject)}
		res = append(res, p)
	}
	if len(filters.Has) > 0 {
		p := Property{PropertyHas, joinOR(filters.Has)}
		res = append(res, p)
	}
	return res, nil
}

func generateActions(actions Actions) ([]Property, error) {
	res := []Property{}
	if actions.Archive {
		res = append(res, Property{PropertyArchive, "true"})
	}
	if actions.Delete {
		res = append(res, Property{PropertyDelete, "true"})
	}
	if actions.MarkImportant {
		res = append(res, Property{PropertyMarkImportant, "true"})
	}
	if actions.MarkRead {
		res = append(res, Property{PropertyMarkRead, "true"})
	}
	if len(actions.Category) > 0 {
		cat, err := categoryToSmartLabel(actions.Category)
		if err != nil {
			return nil, err
		}
		res = append(res, Property{PropertyApplyCategory, cat})
	}
	for _, label := range actions.Labels {
		res = append(res, Property{PropertyApplyLabel, label})
	}
	return res, nil
}

func categoryToSmartLabel(cat Category) (string, error) {
	var smartl string
	switch cat {
	case CategoryPersonal:
		smartl = SmartLabelPersonal
	case CategorySocial:
		smartl = SmartLabelSocial
	case CategoryUpdates:
		smartl = SmartLabelNotification
	case CategoryForums:
		smartl = SmartLabelGroup
	case CategoryPromotions:
		smartl = SmartLabelPromo
	default:
		return "", fmt.Errorf("unrecognized category '%s", cat)
	}
	return fmt.Sprintf("^smartlabel_%s", smartl), nil
}

func joinOR(a []string) string {
	if len(a) == 0 {
		return ""
	}
	if len(a) == 1 {
		return a[0]
	}
	return fmt.Sprintf("{%s}", strings.Join(quote(a), " "))
}

func quote(a []string) []string {
	res := make([]string, len(a))
	for i, s := range a {
		if strings.ContainsRune(s, ' ') {
			s = fmt.Sprintf(`"%s"`, s)
		}
		res[i] = s
	}
	return res
}

func combineFiltersActions(filters []Property, actions []Property) []Entry {
	// Since only one label is allowed in the exported entries,
	// we have to create a new entry for each label and use the same filters for each of them
	res := []Entry{}
	curr := copyPropertiesToEntry(filters)
	countLabels := 0
	for _, action := range actions {
		if action.Name == PropertyApplyLabel {
			countLabels++
			if countLabels > 1 {
				// Append the current entry and start with a fresh one
				res = append(res, curr)
				curr = copyPropertiesToEntry(filters)
			}
			countLabels = 1
		}
		curr = append(curr, action)
	}
	// Append the last entry
	res = append(res, curr)

	return res
}

func copyPropertiesToEntry(p []Property) Entry {
	cp := make([]Property, len(p))
	copy(cp, p)
	return Entry(cp)
}