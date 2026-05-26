// Copyright 2026 Bjørn Erik Pedersen
// SPDX-License-Identifier: MIT

package internal

import (
	"testing"
)

// TestMergeIntoPartialMonths verifies that mergeInto fills empty slots in a
// partially-populated array from the parent, without overwriting non-empty
// child values.
func TestMergeIntoPartialMonths(t *testing.T) {
	parent := LocaleConfig{
		MonthsConfig: MonthsConfig{
			MonthsWide: Months{
				"janeiro", "fevereiro", "março", "abril",
				"maio", "junho", "julho", "agosto",
				"setembro", "outubro", "novembro", "dezembro",
			},
			MonthsAbbreviated: Months{
				"jan.", "fev.", "mar.", "abr.",
				"mai.", "jun.", "jul.", "ago.",
				"set.", "out.", "nov.", "dez.",
			},
		},
		WeekDaysConfig: WeekDaysConfig{
			WeekDaysWide: Days{
				"domingo", "segunda-feira", "terça-feira", "quarta-feira",
				"quinta-feira", "sexta-feira", "sábado",
			},
		},
	}

	// Child (e.g. pt-BR) overrides only the first four wide month names
	// and provides no abbreviated names.
	child := LocaleConfig{
		MonthsConfig: MonthsConfig{
			MonthsWide: Months{
				"janeiro (BR)", "fevereiro (BR)", "março (BR)", "abril (BR)",
				"", "", "", "",
				"", "", "", "",
			},
		},
	}

	mergeInto(&child, &parent)

	// First four wide months are from the child.
	for i := 0; i < 4; i++ {
		if child.MonthsWide[i] == parent.MonthsWide[i] {
			t.Errorf("MonthsWide[%d]: expected child override %q, got parent value %q",
				i, child.MonthsWide[i], parent.MonthsWide[i])
		}
	}

	// Remaining eight wide months must come from the parent, not stay empty.
	for i := 4; i < 12; i++ {
		if child.MonthsWide[i] == "" {
			t.Errorf("MonthsWide[%d]: expected parent value %q, got empty string",
				i, parent.MonthsWide[i])
		}
		if child.MonthsWide[i] != parent.MonthsWide[i] {
			t.Errorf("MonthsWide[%d]: expected parent value %q, got %q",
				i, parent.MonthsWide[i], child.MonthsWide[i])
		}
	}

	// Abbreviated months were absent in the child, so all are inherited from the parent.
	for i := 0; i < 12; i++ {
		if child.MonthsAbbreviated[i] != parent.MonthsAbbreviated[i] {
			t.Errorf("MonthsAbbreviated[%d]: expected %q from parent, got %q",
				i, parent.MonthsAbbreviated[i], child.MonthsAbbreviated[i])
		}
	}

	// Weekdays were absent in the child, so all are inherited from the parent.
	for i := 0; i < 7; i++ {
		if child.WeekDaysWide[i] != parent.WeekDaysWide[i] {
			t.Errorf("WeekDaysWide[%d]: expected %q from parent, got %q",
				i, parent.WeekDaysWide[i], child.WeekDaysWide[i])
		}
	}
}

// TestMergeIntoChildPreserved verifies that non-empty child values are never
// overwritten by the parent.
func TestMergeIntoChildPreserved(t *testing.T) {
	parent := LocaleConfig{
		MonthsConfig: MonthsConfig{
			MonthsWide: Months{
				"January", "February", "March", "April",
				"May", "June", "July", "August",
				"September", "October", "November", "December",
			},
		},
		NumberConfig: NumberConfig{
			Decimal: ".",
			Group:   ",",
		},
	}

	child := LocaleConfig{
		MonthsConfig: MonthsConfig{
			MonthsWide: Months{
				"Jan-child", "Feb-child", "March", "April",
				"May", "June", "July", "August",
				"September", "October", "November", "December",
			},
		},
		NumberConfig: NumberConfig{
			Decimal: ",",
			Group:   " ",
		},
	}

	mergeInto(&child, &parent)

	if child.MonthsWide[0] != "Jan-child" {
		t.Errorf("MonthsWide[0]: child value should be preserved, got %q", child.MonthsWide[0])
	}
	if child.MonthsWide[1] != "Feb-child" {
		t.Errorf("MonthsWide[1]: child value should be preserved, got %q", child.MonthsWide[1])
	}
	if child.Decimal != "," {
		t.Errorf("Decimal: child value should be preserved, got %q", child.Decimal)
	}
	if child.Group != " " {
		t.Errorf("Group: child value should be preserved, got %q", child.Group)
	}
}
