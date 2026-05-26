// Copyright 2026 Bjørn Erik Pedersen
// SPDX-License-Identifier: MIT

package internal

import (
	"fmt"
	"iter"
	"log"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/text/unicode/cldr"
)

var (
	TestMode = false
	// testModeLocales = []string{"en", "de", "fr", "zh", "no", "nb", "nb_NO", "nn", "nn_NO"}
	testModeLocales = []string{"en", "de", "de_BE", "fr", "zh", "no", "nb", "nb_NO", "nn", "nn_NO"}
)

func Generate(dataPath string) iter.Seq2[string, LocaleConfig] {
	return func(yeld func(string, LocaleConfig) bool) {
		// load CLDR recourses
		var decoder cldr.Decoder
		decoder.SetDirFilter("main", "supplemental")

		cldrData, err := decoder.DecodePath(dataPath)
		if err != nil {
			log.Fatal(err)
		}

		parentOverrides := BuildParentLocaleMap(cldrData)

		for _, l := range cldrData.Locales() {
			if TestMode && !slices.Contains(testModeLocales, l) {
				continue
			}
			lcfg := ResolveLocale(l, cldrData, parentOverrides)
			if !yeld(l, lcfg) {
				return
			}

		}
	}
}

// BuildMetazoneMap builds a map of IANA timezone name → metazone name
// from the CLDR supplemental metazoneInfo data.
// Only the current (no "to" date) mapping is used.
func BuildMetazoneMap(cldrData *cldr.CLDR) map[string]string {
	m := make(map[string]string)
	sup := cldrData.Supplemental()
	if sup.MetaZones == nil || sup.MetaZones.MetazoneInfo == nil {
		return m
	}
	for _, tz := range sup.MetaZones.MetazoneInfo.Timezone {
		for _, umz := range tz.UsesMetazone {
			if umz.To == "" {
				// Current mapping (no end date).
				m[tz.Type] = umz.Mzone
			}
		}
	}
	return m
}

// BuildParentLocaleMap builds a map of locale -> parent from the CLDR
// supplemental parentLocales data, falling back to the default algorithm
// (strip rightmost component, then "root").
func BuildParentLocaleMap(cldrData *cldr.CLDR) map[string]string {
	m := make(map[string]string)
	sup := cldrData.Supplemental()
	if sup.ParentLocales != nil {
		for _, pl := range sup.ParentLocales.ParentLocale {
			for _, loc := range strings.Fields(pl.Locales) {
				m[loc] = pl.Parent
			}
		}
	}
	return m
}

// parentLocale returns the parent of loc. It consults the explicit override
// map first, then falls back to stripping the rightmost component.
// Returns "" for "root".
func parentLocale(loc string, overrides map[string]string) string {
	if loc == "root" {
		return ""
	}
	if p, ok := overrides[loc]; ok {
		return p
	}
	if i := strings.LastIndex(loc, "_"); i >= 0 {
		return loc[:i]
	}
	return "root"
}

// mergeInto copies non-zero fields from src into dst where dst's field is
// the zero value. This works for any struct fields (slices, arrays, strings,
// etc.) and is the single place inheritance is applied — adding new fields
// to localeConfig automatically participates in inheritance.
// Map fields are merged key-by-key (parent keys fill gaps in child).
func mergeInto(dst, src *LocaleConfig) {
	dv := reflect.ValueOf(dst).Elem()
	sv := reflect.ValueOf(src).Elem()
	for i := range dv.NumField() {
		df := dv.Field(i)
		sf := sv.Field(i)
		if df.Kind() == reflect.Struct {
			mergeStruct(df, sf)
		} else if df.Kind() == reflect.Map {
			mergeMap(df, sf)
		} else if df.IsZero() && !sf.IsZero() {
			df.Set(sf)
		}
	}
}

func mergeStruct(dst, src reflect.Value) {
	for i := range dst.NumField() {
		df := dst.Field(i)
		sf := src.Field(i)
		if df.Kind() == reflect.Map {
			mergeMap(df, sf)
		} else if df.Kind() == reflect.Array {
			mergeArray(df, sf)
		} else if df.IsZero() && !sf.IsZero() {
			df.Set(sf)
		}
	}
}

func mergeArray(dst, src reflect.Value) {
	for i := range dst.Len() {
		de := dst.Index(i)
		se := src.Index(i)
		if de.IsZero() && !se.IsZero() {
			de.Set(se)
		}
	}
}

func mergeMap(dst, src reflect.Value) {
	if src.IsNil() {
		return
	}
	if dst.IsNil() {
		dst.Set(reflect.MakeMap(dst.Type()))
	}
	for _, k := range src.MapKeys() {
		if dst.MapIndex(k).IsValid() {
			continue // child already has this key
		}
		dst.SetMapIndex(k, src.MapIndex(k))
	}
}

// extractFromLDML extracts locale data from a raw LDML into a localeConfig.
func extractFromLDML(ldml *cldr.LDML) LocaleConfig {
	var lcfg LocaleConfig
	if ldml == nil {
		return lcfg
	}
	calendar := gregorianCalendar(ldml)
	if calendar != nil {
		extractMonths(calendar, &lcfg)
		extractDays(calendar, &lcfg)
		extractDateTimeFormats(calendar, &lcfg)
	}
	extractNumbers(ldml, &lcfg)
	extractCurrencies(ldml, &lcfg)
	extractTimeZoneNames(ldml, &lcfg)
	return lcfg
}

func gregorianCalendar(ldml *cldr.LDML) *cldr.Calendar {
	if ldml.Dates == nil || ldml.Dates.Calendars == nil {
		return nil
	}
	for _, cal := range ldml.Dates.Calendars.Calendar {
		if cal.Type == "gregorian" {
			return cal
		}
	}
	return nil
}

func extractMonths(calendar *cldr.Calendar, lcfg *LocaleConfig) {
	if calendar.Months == nil {
		return
	}
	for _, ctx := range calendar.Months.MonthContext {
		if ctx.Type != "format" {
			continue
		}
		for _, width := range ctx.MonthWidth {
			var months Months
			for _, m := range width.Month {
				if m.Yeartype != "" {
					continue
				}
				idx := 0
				fmt.Sscanf(m.Type, "%d", &idx)
				if idx >= 1 && idx <= 12 {
					months[idx-1] = m.Data()
				}
			}
			switch width.Type {
			case "abbreviated":
				lcfg.MonthsAbbreviated = months
			case "narrow":
				lcfg.MonthsNarrow = months
			case "wide":
				lcfg.MonthsWide = months
			}
		}
	}
}

var dayIndex = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3,
	"thu": 4, "fri": 5, "sat": 6,
}

func extractDays(calendar *cldr.Calendar, lcfg *LocaleConfig) {
	if calendar.Days == nil {
		return
	}
	for _, ctx := range calendar.Days.DayContext {
		if ctx.Type != "format" {
			continue
		}
		for _, width := range ctx.DayWidth {
			var days Days
			for _, d := range width.Day {
				if i, ok := dayIndex[d.Type]; ok {
					days[i] = d.Data()
				}
			}
			switch width.Type {
			case "abbreviated":
				lcfg.WeekDaysAbbreviated = days
			case "narrow":
				lcfg.WeekDaysNarrow = days
			case "short":
				lcfg.WeekDaysShort = days
			case "wide":
				lcfg.WeekDaysWide = days
			}
		}
	}
}

// normalizeDateShortYear replaces a standalone single 'y' in a short date pattern
// with 'yy' so that the formatter produces 2-digit years for short date formats.
func normalizeDateShortYear(pattern string) string {
	// Find runs of 'y'. If there's exactly one 'y' (not 'yy' or more), replace with 'yy'.
	i := 0
	var result []byte
	changed := false
	for i < len(pattern) {
		if pattern[i] == '\'' {
			result = append(result, pattern[i])
			i++
			for i < len(pattern) && pattern[i] != '\'' {
				result = append(result, pattern[i])
				i++
			}
			if i < len(pattern) {
				result = append(result, pattern[i])
				i++
			}
			continue
		}
		if pattern[i] == 'y' {
			start := i
			for i < len(pattern) && pattern[i] == 'y' {
				i++
			}
			if i-start == 1 {
				result = append(result, 'y', 'y')
				changed = true
			} else {
				result = append(result, pattern[start:i]...)
			}
			continue
		}
		result = append(result, pattern[i])
		i++
	}
	if !changed {
		return pattern
	}
	return string(result)
}

func extractDateTimeFormats(calendar *cldr.Calendar, lcfg *LocaleConfig) {
	if calendar.DateFormats != nil {
		for _, dfl := range calendar.DateFormats.DateFormatLength {
			for _, df := range dfl.DateFormat {
				for _, p := range df.Pattern {
					if p.Alt != "" {
						continue
					}
					switch dfl.Type {
					case "full":
						lcfg.DateFull = p.Data()
					case "long":
						lcfg.DateLong = p.Data()
					case "medium":
						lcfg.DateMedium = p.Data()
					case "short":
						lcfg.DateShort = normalizeDateShortYear(p.Data())
					}
				}
			}
		}
	}
	if calendar.TimeFormats != nil {
		for _, tfl := range calendar.TimeFormats.TimeFormatLength {
			for _, tf := range tfl.TimeFormat {
				for _, p := range tf.Pattern {
					if p.Alt != "" {
						continue
					}
					switch tfl.Type {
					case "full":
						lcfg.TimeFull = p.Data()
					case "long":
						lcfg.TimeLong = p.Data()
					case "medium":
						lcfg.TimeMedium = p.Data()
					case "short":
						lcfg.TimeShort = p.Data()
					}
				}
			}
		}
	}
}

func extractTimeZoneNames(ldml *cldr.LDML, lcfg *LocaleConfig) {
	if ldml.Dates == nil || ldml.Dates.TimeZoneNames == nil {
		return
	}
	tzn := ldml.Dates.TimeZoneNames
	for _, mz := range tzn.Metazone {
		if len(mz.Long) == 0 {
			continue
		}
		long := mz.Long[0]
		var standard, daylight string
		if len(long.Standard) > 0 {
			standard = long.Standard[0].Data()
		}
		if len(long.Daylight) > 0 {
			daylight = long.Daylight[0].Data()
		}
		if standard == "" && daylight == "" {
			continue
		}
		if lcfg.TZNames == nil {
			lcfg.TZNames = make(TimeZoneNames)
		}
		lcfg.TZNames[mz.Type] = [2]string{standard, daylight}
	}
}

func extractNumbers(ldml *cldr.LDML, lcfg *LocaleConfig) {
	if ldml.Numbers == nil {
		return
	}
	// Find symbols for the "latn" number system (or unspecified, which defaults to latn).
	for _, sym := range ldml.Numbers.Symbols {
		if sym.NumberSystem != "" && sym.NumberSystem != "latn" {
			continue
		}
		if len(sym.Decimal) > 0 {
			lcfg.Decimal = sym.Decimal[0].Data()
		}
		if len(sym.Group) > 0 {
			lcfg.Group = sym.Group[0].Data()
		}
		if len(sym.MinusSign) > 0 {
			lcfg.Minus = sym.MinusSign[0].Data()
		}
		if len(sym.PercentSign) > 0 {
			lcfg.Percent = sym.PercentSign[0].Data()
		}
		if len(sym.PerMille) > 0 {
			lcfg.PerMille = sym.PerMille[0].Data()
		}
		if len(sym.PlusSign) > 0 {
			lcfg.Plus = sym.PlusSign[0].Data()
		}
		break
	}

	// Extract percent format pattern.
	for _, pf := range ldml.Numbers.PercentFormats {
		if pf.NumberSystem != "" && pf.NumberSystem != "latn" {
			continue
		}
		for _, pfl := range pf.PercentFormatLength {
			if pfl.Type != "" {
				continue
			}
			for _, pfmt := range pfl.PercentFormat {
				for _, p := range pfmt.Pattern {
					if p.Alt != "" || p.Count != "" {
						continue
					}
					lcfg.PercentPattern = p.Data()
				}
			}
		}
		break
	}
}

func extractCurrencies(ldml *cldr.LDML, lcfg *LocaleConfig) {
	if ldml.Numbers == nil {
		return
	}

	// Extract accounting format pattern from latn currency formats.
	for _, cf := range ldml.Numbers.CurrencyFormats {
		if cf.NumberSystem != "" && cf.NumberSystem != "latn" {
			continue
		}
		for _, cfl := range cf.CurrencyFormatLength {
			if cfl.Type != "" {
				continue // skip "short" etc., we want the default length
			}
			for _, cfmt := range cfl.CurrencyFormat {
				for _, p := range cfmt.Pattern {
					if p.Alt != "" || p.Count != "" {
						continue
					}
					switch cfmt.Type {
					case "standard":
						lcfg.StandardPattern = p.Data()
					case "accounting":
						lcfg.AccountingPattern = p.Data()
					}
				}
			}
		}
		break
	}

	// Extract currency symbols.
	if ldml.Numbers.Currencies != nil {
		for _, cur := range ldml.Numbers.Currencies.Currency {
			if len(cur.Symbol) > 0 {
				if lcfg.CurrencySymbols == nil {
					lcfg.CurrencySymbols = make(map[string]string)
				}
				lcfg.CurrencySymbols[cur.Type] = cur.Symbol[0].Data()
			}
		}
	}
}

// ResolveLocale builds a fully inherited localeConfig by walking up the
// parent chain (e.g. nn -> no -> root) and merging.
func ResolveLocale(loc string, cldrData *cldr.CLDR, parentOverrides map[string]string) LocaleConfig {
	lcfg := extractFromLDML(cldrData.RawLDML(loc))
	for p := parentLocale(loc, parentOverrides); p != ""; p = parentLocale(p, parentOverrides) {
		parentCfg := extractFromLDML(cldrData.RawLDML(p))
		mergeInto(&lcfg, &parentCfg)
	}
	return lcfg
}
