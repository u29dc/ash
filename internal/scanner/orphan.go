package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/charlievieth/fastwalk"
	"howett.net/plist"

	"ash/internal/safety"
)

// ConfidenceLevel indicates how confident we are that a leftover belongs to an app.
type ConfidenceLevel int

const (
	ConfidenceHigh   ConfidenceLevel = iota // Exact bundle ID match
	ConfidenceMedium                        // App name match
	ConfidenceLow                           // Company name only
)

var (
	knownLeftoverSuffixes = []string{
		".plist",
		".savedstate",
		".binarycookies",
	}
	genericAppTerms = map[string]struct{}{
		"agent":    {},
		"app":      {},
		"cache":    {},
		"client":   {},
		"daemon":   {},
		"desktop":  {},
		"helper":   {},
		"launcher": {},
		"service":  {},
		"support":  {},
	}
)

const locationGroupContainers = "Group Containers"

// AppInfo contains information extracted from an app bundle.
type AppInfo struct {
	BundleID    string
	Name        string
	DisplayName string
	Company     string
	Path        string
}

// Leftover represents an orphaned app file or directory.
type Leftover struct {
	Path         string
	Size         int64
	AppInfo      *AppInfo
	Confidence   ConfidenceLevel
	Location     string // Which Library subdirectory
	ManualReview bool
	SharedData   bool
}

// RiskLevel maps deep-scan leftovers onto the live cleanup risk model.
func (l Leftover) RiskLevel() RiskLevel {
	if l.ManualReview || l.SharedData {
		return RiskDangerous
	}
	return RiskCaution
}

// OrphanFinder detects leftover files from uninstalled applications.
type OrphanFinder struct {
	pathConfig      *PathConfig
	appSearchRoots  []string
	leftoverFolders []string
}

type installedAppIndex struct {
	bundleIDs map[string]*AppInfo
	names     map[string]struct{}
	companies map[string]struct{}
}

type matchKind int

const (
	matchNone matchKind = iota
	matchBundleID
	matchName
	matchCompany
)

type matchResult struct {
	confidence ConfidenceLevel
	kind       matchKind
	term       string
	matched    bool
}

// NewOrphanFinder creates a new OrphanFinder for the current user.
func NewOrphanFinder() (*OrphanFinder, error) {
	cfg, err := NewPathConfig()
	if err != nil {
		return nil, err
	}
	return newOrphanFinder(cfg), nil
}

// NewOrphanFinderWithConfig creates a new OrphanFinder with explicit path configuration.
func NewOrphanFinderWithConfig(cfg *PathConfig, appRoots []string) (*OrphanFinder, error) {
	if cfg == nil {
		return nil, errors.New("path config is required")
	}

	finder := newOrphanFinder(cfg)
	if len(appRoots) > 0 {
		finder.appSearchRoots = dedupePaths(appRoots)
	}

	return finder, nil
}

// NewOrphanFinderForHome creates a new OrphanFinder rooted at the provided home directory.
func NewOrphanFinderForHome(homeDir string) *OrphanFinder {
	return newOrphanFinder(&PathConfig{
		HomeDir: homeDir,
		LibDir:  filepath.Join(homeDir, "Library"),
	})
}

func newOrphanFinder(cfg *PathConfig) *OrphanFinder {
	return &OrphanFinder{
		pathConfig:      cfg,
		appSearchRoots:  appSearchRoots(cfg.HomeDir),
		leftoverFolders: cfg.AppLeftoverLocations(),
	}
}

func appSearchRoots(homeDir string) []string {
	candidates := []string{
		"/Applications",
		"/System/Applications",
		filepath.Join(homeDir, "Applications"),
	}

	return dedupePaths(candidates)
}

func dedupePaths(candidates []string) []string {
	seen := make(map[string]struct{}, len(candidates))
	roots := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		cleaned := filepath.Clean(candidate)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		roots = append(roots, cleaned)
	}

	return roots
}

// ExtractAppInfo extracts bundle information from an app bundle.
func (o *OrphanFinder) ExtractAppInfo(appPath string) (*AppInfo, error) {
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return nil, err
	}

	var info struct {
		CFBundleIdentifier  string `plist:"CFBundleIdentifier"`
		CFBundleName        string `plist:"CFBundleName"`
		CFBundleDisplayName string `plist:"CFBundleDisplayName"`
	}

	if _, err := plist.Unmarshal(data, &info); err != nil {
		info.CFBundleIdentifier = plistStringValue(data, "CFBundleIdentifier")
		info.CFBundleName = plistStringValue(data, "CFBundleName")
		info.CFBundleDisplayName = plistStringValue(data, "CFBundleDisplayName")
		if info.CFBundleIdentifier == "" {
			return nil, err
		}
	}
	if info.CFBundleIdentifier == "" {
		info.CFBundleIdentifier = plistStringValue(data, "CFBundleIdentifier")
	}
	if info.CFBundleName == "" {
		info.CFBundleName = plistStringValue(data, "CFBundleName")
	}
	if info.CFBundleDisplayName == "" {
		info.CFBundleDisplayName = plistStringValue(data, "CFBundleDisplayName")
	}

	appInfo := &AppInfo{
		BundleID:    info.CFBundleIdentifier,
		Name:        info.CFBundleName,
		DisplayName: info.CFBundleDisplayName,
		Path:        appPath,
	}

	parts := strings.Split(info.CFBundleIdentifier, ".")
	if len(parts) >= 2 {
		appInfo.Company = parts[1]
	}
	if appInfo.Name == "" {
		appInfo.Name = deriveAppName(appInfo.BundleID)
	}

	return appInfo, nil
}

// GenerateSearchTerms creates search terms from app info.
func (o *OrphanFinder) GenerateSearchTerms(info *AppInfo) []string {
	terms := make([]string, 0, 6)
	seen := make(map[string]struct{}, 6)

	appendTerm := func(term string) {
		term = strings.TrimSpace(term)
		if term == "" {
			return
		}
		if _, ok := seen[term]; ok {
			return
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}

	appendTerm(info.BundleID)
	appendTerm(info.Name)
	appendTerm(strings.ToLower(info.Name))
	appendTerm(info.DisplayName)
	appendTerm(strings.ToLower(info.DisplayName))
	appendTerm(info.Company)

	return terms
}

// FindLeftovers finds orphaned files for a given app using a background context.
func (o *OrphanFinder) FindLeftovers(info *AppInfo) ([]Leftover, error) {
	return o.FindLeftoversContext(context.Background(), info)
}

// FindLeftoversContext finds orphaned files for a given app and stops promptly on cancellation.
func (o *OrphanFinder) FindLeftoversContext(ctx context.Context, info *AppInfo) ([]Leftover, error) {
	index, err := o.buildInstalledAppIndex(ctx)
	if err != nil {
		return nil, err
	}
	return o.findLeftovers(ctx, info, index)
}

// FindAllOrphans scans for leftovers from apps that are no longer installed using a background context.
func (o *OrphanFinder) FindAllOrphans() ([]Leftover, error) {
	return o.FindAllOrphansContext(context.Background())
}

// FindAllOrphansContext scans for leftovers from apps that are no longer installed.
func (o *OrphanFinder) FindAllOrphansContext(ctx context.Context) ([]Leftover, error) {
	index, err := o.buildInstalledAppIndex(ctx)
	if err != nil {
		return nil, err
	}

	candidates, err := o.discoverCandidates(ctx, index)
	if err != nil {
		return nil, err
	}

	byPath := make(map[string]Leftover, len(candidates)*2)
	for _, candidate := range candidates {
		leftovers, err := o.findLeftovers(ctx, candidate, index)
		if err != nil {
			return nil, err
		}
		for _, leftover := range leftovers {
			existing, ok := byPath[leftover.Path]
			if !ok {
				byPath[leftover.Path] = leftover
				continue
			}
			byPath[leftover.Path] = mergeLeftover(existing, leftover)
		}
	}

	results := make([]Leftover, 0, len(byPath))
	for _, leftover := range byPath {
		results = append(results, leftover)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Confidence != results[j].Confidence {
			return results[i].Confidence < results[j].Confidence
		}
		return results[i].Path < results[j].Path
	})

	return results, nil
}

func (o *OrphanFinder) buildInstalledAppIndex(ctx context.Context) (*installedAppIndex, error) {
	index := &installedAppIndex{
		bundleIDs: make(map[string]*AppInfo),
		names:     make(map[string]struct{}),
		companies: make(map[string]struct{}),
	}

	for _, root := range o.appSearchRoots {
		if err := walkApps(ctx, root, func(appPath string) error {
			info, err := o.ExtractAppInfo(appPath)
			if err != nil {
				return nil
			}
			index.add(info)
			return nil
		}); err != nil {
			return nil, err
		}
	}

	return index, nil
}

func walkApps(ctx context.Context, root string, visit func(appPath string) error) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) || os.IsPermission(walkErr) {
				return nil
			}
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !entry.IsDir() {
			return nil
		}

		if path != root && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}

		if strings.HasSuffix(strings.ToLower(entry.Name()), ".app") {
			if err := visit(path); err != nil {
				return err
			}
			return filepath.SkipDir
		}

		return nil
	})
}

func (o *OrphanFinder) discoverCandidates(ctx context.Context, index *installedAppIndex) ([]*AppInfo, error) {
	candidates := make(map[string]*AppInfo)

	for _, location := range o.leftoverFolders {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		entries, err := os.ReadDir(location)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			path := filepath.Join(location, entry.Name())
			if !safety.IsSafePath(path) {
				continue
			}

			bundleID := o.extractBundleID(entry.Name())
			if bundleID == "" {
				continue
			}
			if safety.IsProtectedApp(bundleID) || index.hasBundleID(bundleID) {
				continue
			}

			if _, ok := candidates[bundleID]; ok {
				continue
			}

			candidates[bundleID] = &AppInfo{
				BundleID:    bundleID,
				Name:        deriveAppName(bundleID),
				DisplayName: deriveAppName(bundleID),
				Company:     companyFromBundleID(bundleID),
			}
		}
	}

	bundleIDs := make([]string, 0, len(candidates))
	for bundleID := range candidates {
		bundleIDs = append(bundleIDs, bundleID)
	}
	sort.Strings(bundleIDs)

	results := make([]*AppInfo, 0, len(bundleIDs))
	for _, bundleID := range bundleIDs {
		results = append(results, candidates[bundleID])
	}
	return results, nil
}

func (o *OrphanFinder) findLeftovers(
	ctx context.Context,
	info *AppInfo,
	index *installedAppIndex,
) ([]Leftover, error) {
	if info == nil {
		return nil, nil
	}

	leftovers := make([]Leftover, 0, len(o.leftoverFolders))

	for _, location := range o.leftoverFolders {
		select {
		case <-ctx.Done():
			return leftovers, ctx.Err()
		default:
		}

		entries, err := os.ReadDir(location)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			select {
			case <-ctx.Done():
				return leftovers, ctx.Err()
			default:
			}

			locationName := filepath.Base(location)
			match := o.matchEntry(locationName, entry.Name(), info)
			if !match.matched {
				continue
			}
			if match.kind != matchCompany && index.contains(match.kind, match.term) {
				continue
			}

			path := filepath.Join(location, entry.Name())
			if !safety.IsSafePath(path) {
				continue
			}
			if entryBundleID := o.extractBundleID(entry.Name()); entryBundleID != "" {
				if safety.IsProtectedApp(entryBundleID) || index.hasBundleID(entryBundleID) {
					continue
				}
			}

			size, err := o.getSize(ctx, path, entry.IsDir())
			if err != nil {
				return leftovers, err
			}

			leftover := Leftover{
				Path:       path,
				Size:       size,
				AppInfo:    info,
				Confidence: match.confidence,
				Location:   locationName,
			}
			leftover.SharedData = locationName == locationGroupContainers
			leftover.ManualReview = requiresManualReview(locationName, match.confidence)
			leftovers = append(leftovers, leftover)
		}
	}

	return leftovers, nil
}

func (o *OrphanFinder) matchEntry(location, name string, info *AppInfo) matchResult {
	if info == nil {
		return matchResult{}
	}

	bundleID := o.extractBundleID(name)
	if bundleID != "" {
		switch {
		case info.BundleID != "" && bundleID == info.BundleID:
			return matchResult{
				confidence: ConfidenceHigh,
				kind:       matchBundleID,
				term:       info.BundleID,
				matched:    true,
			}
		case info.BundleID != "" && strings.Contains(strings.ToLower(trimKnownSuffixes(name)), strings.ToLower(info.BundleID)):
			return matchResult{
				confidence: ConfidenceHigh,
				kind:       matchBundleID,
				term:       info.BundleID,
				matched:    true,
			}
		default:
			// A leftover that already carries a bundle ID should not be reassigned
			// using weaker name/company heuristics from unrelated apps.
			return matchResult{}
		}
	}

	nameNorm := normalizeSearchTerm(name)
	if location == "Application Support" {
		for _, term := range appNameTerms(info) {
			termNorm := normalizeSearchTerm(term)
			if termNorm == "" {
				continue
			}
			if strings.Contains(nameNorm, termNorm) {
				return matchResult{
					confidence: ConfidenceMedium,
					kind:       matchName,
					term:       termNorm,
					matched:    true,
				}
			}
		}
	}

	if location == "Group Containers" {
		company := normalizeSearchTerm(info.Company)
		if company != "" && len(company) >= 4 && strings.Contains(nameNorm, company) {
			return matchResult{
				confidence: ConfidenceLow,
				kind:       matchCompany,
				term:       company,
				matched:    true,
			}
		}
	}

	return matchResult{}
}

func (o *OrphanFinder) getSize(ctx context.Context, path string, isDir bool) (int64, error) {
	if !isDir {
		info, err := os.Stat(path)
		if err != nil {
			return 0, err
		}
		return info.Size(), nil
	}

	var size atomic.Int64
	conf := fastwalk.Config{Follow: false}
	err := fastwalk.Walk(&conf, path, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) || os.IsPermission(walkErr) {
				return nil
			}
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err == nil {
			size.Add(info.Size())
		}
		return nil
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		return size.Load(), err
	}
	if ctx.Err() != nil {
		return size.Load(), ctx.Err()
	}

	return size.Load(), nil
}

func (i *installedAppIndex) add(info *AppInfo) {
	if info == nil {
		return
	}
	if info.BundleID != "" {
		i.bundleIDs[info.BundleID] = info
	}
	for _, term := range []string{info.Name, info.DisplayName} {
		norm := normalizeSearchTerm(term)
		if norm != "" {
			i.names[norm] = struct{}{}
		}
	}
	if company := normalizeSearchTerm(info.Company); company != "" {
		i.companies[company] = struct{}{}
	}
}

func (i *installedAppIndex) hasBundleID(bundleID string) bool {
	if i == nil || bundleID == "" {
		return false
	}
	_, ok := i.bundleIDs[bundleID]
	return ok
}

func (i *installedAppIndex) contains(kind matchKind, term string) bool {
	if i == nil || term == "" {
		return false
	}
	switch kind {
	case matchBundleID:
		return i.hasBundleID(term)
	case matchName:
		_, ok := i.names[term]
		return ok
	case matchCompany:
		_, ok := i.companies[term]
		return ok
	default:
		return false
	}
}

func mergeLeftover(existing, candidate Leftover) Leftover {
	if existing.Path == "" {
		return candidate
	}
	if candidate.Path == "" {
		return existing
	}
	if existing.AppInfo != nil && candidate.AppInfo != nil && existing.AppInfo.BundleID != candidate.AppInfo.BundleID {
		switch {
		case candidate.Confidence < existing.Confidence:
			return candidate
		case candidate.Confidence > existing.Confidence:
			return existing
		case candidate.Confidence == existing.Confidence:
			existing.AppInfo = nil
			existing.ManualReview = true
			existing.Confidence = maxConfidence(existing.Confidence, candidate.Confidence)
			if candidate.SharedData {
				existing.SharedData = true
			}
			return existing
		}
	}
	if candidate.SharedData {
		existing.SharedData = true
		existing.ManualReview = true
	}
	if candidate.ManualReview {
		existing.ManualReview = true
	}
	if candidate.Confidence < existing.Confidence && existing.AppInfo != nil {
		existing.Confidence = candidate.Confidence
	}
	return existing
}

func requiresManualReview(location string, confidence ConfidenceLevel) bool {
	switch location {
	case locationGroupContainers, "Application Support":
		return true
	case "Containers":
		return confidence != ConfidenceHigh
	default:
		return confidence == ConfidenceLow
	}
}

func appNameTerms(info *AppInfo) []string {
	terms := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	for _, candidate := range []string{info.Name, info.DisplayName, deriveAppName(info.BundleID)} {
		norm := normalizeSearchTerm(candidate)
		if norm == "" {
			continue
		}
		if _, ok := genericAppTerms[norm]; ok {
			continue
		}
		if len(norm) < 4 {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		terms = append(terms, candidate)
	}
	return terms
}

func companyFromBundleID(bundleID string) string {
	parts := strings.Split(bundleID, ".")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func deriveAppName(bundleID string) string {
	parts := strings.Split(bundleID, ".")
	if len(parts) == 0 {
		return ""
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return ""
	}
	last = strings.NewReplacer("-", " ", "_", " ").Replace(last)
	return strings.Join(strings.Fields(last), " ")
}

func normalizeSearchTerm(value string) string {
	value = strings.TrimSpace(trimKnownSuffixes(value))
	if value == "" {
		return ""
	}
	value = strings.ToLower(value)
	value = strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func trimKnownSuffixes(name string) string {
	trimmed := strings.TrimSpace(name)
	lower := strings.ToLower(trimmed)
	for _, suffix := range knownLeftoverSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return trimmed[:len(trimmed)-len(suffix)]
		}
	}
	return trimmed
}

func plistStringValue(data []byte, key string) string {
	keyTag := "<key>" + key + "</key>"
	content := string(data)
	idx := strings.Index(content, keyTag)
	if idx == -1 {
		return ""
	}
	content = content[idx+len(keyTag):]
	start := strings.Index(content, "<string>")
	end := strings.Index(content, "</string>")
	if start == -1 || end == -1 || end <= start+len("<string>") {
		return ""
	}
	return strings.TrimSpace(content[start+len("<string>") : end])
}

func (o *OrphanFinder) extractBundleID(name string) string {
	trimmed := trimKnownSuffixes(name)
	parts := strings.Split(trimmed, ".")

	if strings.HasPrefix(trimmed, "group.") {
		candidate := strings.TrimPrefix(trimmed, "group.")
		if looksLikeBundleID(candidate) {
			return candidate
		}
	}

	if len(parts) >= 4 && looksLikeTeamID(parts[0]) {
		candidate := strings.Join(parts[1:], ".")
		if looksLikeBundleID(candidate) {
			return candidate
		}
	}

	if looksLikeBundleID(trimmed) {
		return trimmed
	}

	for i := 1; i <= len(parts)-3; i++ {
		candidate := strings.Join(parts[i:], ".")
		if looksLikeBundleID(candidate) {
			return candidate
		}
	}

	return ""
}

func looksLikeBundleID(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) < 3 {
		return false
	}
	if parts[0] != strings.ToLower(parts[0]) {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
				return false
			}
		}
	}
	return true
}

func looksLikeTeamID(value string) bool {
	if len(value) < 4 {
		return false
	}
	for _, r := range value {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func maxConfidence(a, b ConfidenceLevel) ConfidenceLevel {
	if a > b {
		return a
	}
	return b
}
