// Migrations: declarative payloads, window resolve, deterministic verify
// (migrations spec, DR-0008).

package core

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const MigrationsDir = "migrations"

var (
	migrationKinds = []string{"mechanical", "additive", "removal", "structural", "convention"}
	JudgmentKinds  = []string{"structural", "convention"}
	checkKinds     = []string{"file-absent", "file-present", "tool"}
	migIDRx        = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// LoadMigrationEntries: top-level migrations/*.yml only.
func LoadMigrationEntries(publisherRoot string) ([]map[string]any, error) {
	d := filepath.Join(publisherRoot, MigrationsDir)
	paths, _ := filepath.Glob(filepath.Join(d, "*.yml"))
	sort.Strings(paths)
	var entries []map[string]any
	for _, f := range paths {
		data, err := LoadYAML(f)
		if err != nil {
			return nil, err
		}
		var errs []string
		if !schemaVersionIs(data, 1) {
			errs = append(errs, "schema_version must be 1")
		}
		if !migIDRx.MatchString(getStr(data, "id")) {
			errs = append(errs, "id must be kebab-case")
		}
		if _, ok := ParseVersion(getStr(data, "applies_from"), "rc"); !ok {
			errs = append(errs, "applies_from must be release-shaped")
		}
		if !contains(migrationKinds, getStr(data, "kind")) {
			errs = append(errs, fmt.Sprintf("kind must be one of %v", migrationKinds))
		}
		ver := getMap(data, "verification")
		if len(getList(ver, "must_be_absent")) == 0 &&
			len(getList(ver, "must_be_present")) == 0 &&
			len(getList(ver, "checks")) == 0 {
			errs = append(errs, "verification must declare at least one obligation")
		}
		for _, chk := range getList(ver, "checks") {
			cm, _ := chk.(map[string]any)
			if !contains(checkKinds, getStr(cm, "kind")) {
				errs = append(errs, fmt.Sprintf("unknown check kind %q", getStr(cm, "kind")))
			}
		}
		if len(errs) > 0 {
			return nil, Usagef("%s: %s", f, strings.Join(errs, "; "))
		}
		data["_file"] = filepath.Base(f)
		entries = append(entries, data)
	}
	return entries, nil
}

// ResolveMigrations: applicable entries in (pinned, target] for the
// profile, plus the aggregated obligations document.
func ResolveMigrations(entries []map[string]any, pinned, target, profile string,
	kinds []string) ([]map[string]any, map[string]any, error) {
	if kinds == nil {
		kinds = JudgmentKinds
	}
	var applicable []map[string]any
	for _, e := range entries {
		if !contains(kinds, getStr(e, "kind")) {
			continue
		}
		in, err := InWindow(pinned, getStr(e, "applies_from"), target)
		if err != nil {
			return nil, nil, err
		}
		if !in {
			continue
		}
		profs, _ := strList(e["profiles"])
		if len(profs) == 0 {
			profs = []string{"*"}
		}
		if !contains(profs, "*") && (profile == "" || !contains(profs, profile)) {
			continue
		}
		applicable = append(applicable, e)
	}
	sort.SliceStable(applicable, func(i, j int) bool {
		a, _ := RequireVersion(getStr(applicable[i], "applies_from"))
		b, _ := RequireVersion(getStr(applicable[j], "applies_from"))
		return a.Less(b)
	})
	agg := map[string]any{
		"must_be_absent": []any{}, "must_be_present": []any{}, "checks": []any{},
	}
	for _, e := range applicable {
		ver := getMap(e, "verification")
		for _, key := range []string{"must_be_absent", "must_be_present", "checks"} {
			agg[key] = append(agg[key].([]any), getList(ver, key)...)
		}
	}
	return applicable, agg, nil
}

// -- verify (consumer PR path) ---------------------------------------------------

type VerifyReport struct {
	Failures []string
	Checked  int
}

func trackedFiles(consumerRoot string) ([]string, error) {
	out, err := RunGit([]string{"ls-files"}, consumerRoot)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// VerifyMigrations: deterministic obligation check over tracked files.
// Zero obligations is a green no-op (migrations §4). Uses the same glob
// matcher as resolve/gate (PathMatch).
func VerifyMigrations(consumerRoot string, obligations map[string]any) (*VerifyReport, error) {
	report := &VerifyReport{}
	var files []string
	listed := false // lazily listed: zero obligations must not need git
	tracked := func() ([]string, error) {
		if !listed {
			var err error
			files, err = trackedFiles(consumerRoot)
			if err != nil {
				return nil, err
			}
			listed = true
		}
		return files, nil
	}
	absent, _ := strList(obligations["must_be_absent"])
	for _, glob := range absent {
		report.Checked++
		fs, err := tracked()
		if err != nil {
			return nil, err
		}
		var hits []string
		for _, f := range fs {
			if PathMatch(f, glob) {
				hits = append(hits, f)
			}
		}
		if len(hits) > 0 {
			report.Failures = append(report.Failures, fmt.Sprintf(
				"must_be_absent %q matches %d file(s), e.g. %s",
				glob, len(hits), hits[0]))
		}
	}
	present, _ := strList(obligations["must_be_present"])
	for _, glob := range present {
		report.Checked++
		fs, err := tracked()
		if err != nil {
			return nil, err
		}
		found := false
		for _, f := range fs {
			if PathMatch(f, glob) {
				found = true
				break
			}
		}
		if !found {
			report.Failures = append(report.Failures,
				fmt.Sprintf("must_be_present %q matches nothing", glob))
		}
	}
	checks, _ := obligations["checks"].([]any)
	for _, raw := range checks {
		report.Checked++
		chk, _ := raw.(map[string]any)
		kind := getStr(chk, "kind")
		path := getStr(chk, "path")
		full := filepath.Join(consumerRoot, filepath.FromSlash(path))
		switch kind {
		case "file-absent":
			if _, err := os.Stat(full); err == nil {
				report.Failures = append(report.Failures,
					fmt.Sprintf("file-absent: %s exists", path))
			}
		case "file-present":
			if _, err := os.Stat(full); err != nil {
				report.Failures = append(report.Failures,
					fmt.Sprintf("file-present: %s missing", path))
			}
		case "tool":
			// Executes a manifest-tracked, gate-verified vendored tool —
			// never inline shell from upstream (migrations spec §5).
			if fi, err := os.Stat(full); err != nil || fi.IsDir() {
				report.Failures = append(report.Failures,
					fmt.Sprintf("tool missing: %s", path))
				continue
			}
			args, _ := strList(chk["args"])
			cmd := exec.Command(full, args...)
			cmd.Dir = consumerRoot
			if err := cmd.Run(); err != nil {
				code := 1
				if ee, ok := err.(*exec.ExitError); ok {
					code = ee.ExitCode()
				}
				report.Failures = append(report.Failures,
					fmt.Sprintf("tool %s exited %d", path, code))
			}
		default:
			report.Failures = append(report.Failures,
				fmt.Sprintf("unknown check kind %q", kind))
		}
	}
	return report, nil
}

// LoadObligations: from an inline JSON string or an @file reference.
func LoadObligations(source string) (map[string]any, error) {
	text := source
	if strings.HasPrefix(source, "@") {
		data, err := os.ReadFile(source[1:])
		if err != nil {
			return nil, Usagef("cannot read %s: %v", source[1:], err)
		}
		text = string(data)
	}
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return nil, Usagef("obligations are not valid JSON: %v", err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, Usagef("obligations must be a JSON object")
	}
	return m, nil
}
