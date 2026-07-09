// Consumer slice config: .vendkit/<slice>.yml (DR-0012, onboarding spec §1).

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	SCMValues    = []string{"github", "azure-repos"}
	CIValues     = []string{"github-actions", "azure-pipelines", "none"}
	HandlerKinds = []string{"pr", "handoff", "fact-verify"}
)

type HandlerSpec struct {
	Exec     []string
	DedupKey string
}

type SliceConfig struct {
	SliceName       string
	PublisherSCM    string
	PublisherRepo   string
	SCM             string
	CI              string
	Profile         string
	PinPattern      string
	PinFiles        []string
	Channel         string
	Handlers        map[string]HandlerSpec
	HandoffDedupKey string
	SeedNotes       string
	Attestations    map[string]bool
	Waivers         []map[string]any
	Path            string
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func LoadSliceConfig(path string) (*SliceConfig, error) {
	data, err := LoadYAML(path)
	if err != nil {
		return nil, err
	}
	var errs []string
	if !schemaVersionIs(data, 1) {
		errs = append(errs, "schema_version must be 1")
	}
	name := getStr(data, "slice")
	pub := getMap(data, "publisher")
	pin := getMap(data, "pin")
	watch := getMap(data, "watch")
	if name == "" {
		errs = append(errs, "slice is required")
	}
	if !contains(SCMValues, getStr(pub, "scm")) {
		errs = append(errs, fmt.Sprintf("publisher.scm must be one of %v", SCMValues))
	}
	scm := getStr(data, "scm")
	if !contains(SCMValues, scm) {
		errs = append(errs, fmt.Sprintf("scm must be one of %v", SCMValues))
	}
	ci := getStr(data, "ci")
	if !contains(CIValues, ci) {
		errs = append(errs, fmt.Sprintf("ci must be one of %v", CIValues))
	}
	pinFiles, _ := strList(pin["files"])
	pinPattern := getStr(pin, "pattern")
	if ci == "none" {
		// Manual mode: the manifest's source.release IS the pin.
		if len(pinFiles) > 0 || pinPattern != "" {
			errs = append(errs, "pin block must be empty when ci is 'none'")
		}
	} else if len(pinFiles) == 0 || pinPattern == "" {
		errs = append(errs, "pin.pattern and pin.files (first entry is the "+
			"authoritative read source) are required")
	}
	channel := getStr(watch, "channel")
	if channel == "" {
		channel = "stable"
	}
	if channel != "stable" && channel != "rc" {
		errs = append(errs, "watch.channel must be 'stable' or 'rc'")
	}
	handlers := map[string]HandlerSpec{}
	handlerKeys := []string{}
	for kind := range getMap(data, "handlers") {
		handlerKeys = append(handlerKeys, kind)
	}
	sort.Strings(handlerKeys)
	for _, kind := range handlerKeys {
		spec := getMap(getMap(data, "handlers"), kind)
		if !contains(HandlerKinds, kind) {
			errs = append(errs, fmt.Sprintf(
				"handlers.%s: unknown kind (expected one of %v)", kind, HandlerKinds))
			continue
		}
		command, ok := strList(spec["exec"])
		if !ok || len(command) == 0 {
			errs = append(errs, fmt.Sprintf(
				"handlers.%s.exec must be a non-empty list of strings", kind))
			continue
		}
		handlers[kind] = HandlerSpec{Exec: command, DedupKey: getStr(spec, "dedup_key")}
	}
	seeds := getMap(data, "seeds")
	seedNotes := getStr(seeds, "notes")
	if seedNotes == "" {
		seedNotes = "informational"
	}
	if seedNotes != "informational" && seedNotes != "silent" {
		errs = append(errs, "seeds.notes must be 'informational' or 'silent'")
	}
	if len(errs) > 0 {
		// A half-configured slice must be loud for every command (DR-0012)
		// — this is also what makes the .vendkit/ namespace strict.
		return nil, Usagef("%s: %s", path, strings.Join(errs, "; "))
	}
	attest := map[string]bool{}
	for k, v := range getMap(data, "attestations") {
		if b, ok := v.(bool); ok {
			attest[k] = b
		}
	}
	var waivers []map[string]any
	for _, w := range getList(data, "waivers") {
		if wm, ok := w.(map[string]any); ok {
			waivers = append(waivers, wm)
		}
	}
	dedup := handlers["handoff"].DedupKey
	if dedup == "" {
		dedup = "vendkit-watch-" + name
	}
	return &SliceConfig{
		SliceName:    name,
		PublisherSCM: getStr(pub, "scm"), PublisherRepo: getStr(pub, "repo"),
		SCM: scm, CI: ci,
		Profile:    getStr(data, "profile"),
		PinPattern: pinPattern, PinFiles: pinFiles,
		Channel: channel, Handlers: handlers, HandoffDedupKey: dedup,
		SeedNotes: seedNotes, Attestations: attest, Waivers: waivers,
		Path: path,
	}, nil
}

// DiscoverSliceConfigs: every .vendkit/*.yml is a slice config (DR-0012).
// A stray YAML file there is a usage error, never a silent skip.
func DiscoverSliceConfigs(consumerRoot string) ([]*SliceConfig, error) {
	pattern := filepath.Join(consumerRoot, VendkitDir, "*.yml")
	paths, _ := filepath.Glob(pattern)
	sort.Strings(paths)
	var out []*SliceConfig
	for _, p := range paths {
		cfg, err := LoadSliceConfig(p)
		if err != nil {
			return nil, err
		}
		out = append(out, cfg)
	}
	return out, nil
}

func FindSliceConfig(consumerRoot, sliceName string) (*SliceConfig, error) {
	configs, err := DiscoverSliceConfigs(consumerRoot)
	if err != nil {
		return nil, err
	}
	for _, cfg := range configs {
		if cfg.SliceName == sliceName {
			return cfg, nil
		}
	}
	return nil, nil
}

var pinTailRx = regexp.MustCompile(`^([0-9][0-9A-Za-z.\-]*)`)

// ReadPin: the consumer's pinned-release intent (release-watch spec §2).
func ReadPin(consumerRoot string, cfg *SliceConfig) (string, error) {
	if cfg.CI == "none" {
		mpath := filepath.Join(consumerRoot, VendkitDir, cfg.SliceName+"-manifest.json")
		manifest, err := LoadManifest(mpath)
		if err != nil {
			return "", err
		}
		if release := getStr(getMap(manifest, "source"), "release"); release != "" {
			return release, nil
		}
		return "", Usagef("%s: ci is 'none' and the manifest records no "+
			"source.release — vendor the slice first", cfg.Path)
	}
	pinFile := cfg.PinFiles[0]
	data, err := os.ReadFile(filepath.Join(consumerRoot, filepath.FromSlash(pinFile)))
	if err != nil {
		return "", Usagef("%s: pin file not found: %s", cfg.Path, pinFile)
	}
	seenPattern := false
	for _, line := range strings.Split(string(data), "\n") {
		idx := strings.Index(line, cfg.PinPattern)
		if idx < 0 {
			continue
		}
		seenPattern = true
		// pin.pattern conventionally ends at (and includes) the leading
		// 'v'; the tail is the numeric remainder (e.g. "1.4.2").
		tail := line[idx+len(cfg.PinPattern):]
		if m := pinTailRx.FindStringSubmatch(tail); m != nil {
			candidate := "v" + m[1]
			if _, ok := ParseVersion(candidate, "rc"); ok {
				return candidate, nil
			}
		}
	}
	reason := "pattern not found"
	if seenPattern {
		reason = "pattern found but no parsable version"
	}
	return "", Usagef("%s: pin unreadable in %s: %s", cfg.Path, pinFile, reason)
}
