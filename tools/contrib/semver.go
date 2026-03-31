package contrib

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/saeedalam/agnogo"
)

// Semver returns tools for semantic version parsing and comparison.
func Semver() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "semver_parse", Desc: "Parse a semantic version string into components",
			Params: agnogo.Params{
				"version": {Type: "string", Desc: "Semantic version string (e.g. 1.2.3-beta+build)", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				version := args["version"]
				if version == "" {
					return "", fmt.Errorf("version is required")
				}
				sv, err := parseSemver(version)
				if err != nil {
					return "", err
				}
				result := map[string]any{
					"major":      sv.major,
					"minor":      sv.minor,
					"patch":      sv.patch,
					"prerelease": sv.prerelease,
					"build":      sv.build,
					"string":     sv.String(),
				}
				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
		{
			Name: "semver_compare", Desc: "Compare two semantic versions or check against a constraint",
			Params: agnogo.Params{
				"version":    {Type: "string", Desc: "Version to check (e.g. 1.2.3)", Required: true},
				"other":      {Type: "string", Desc: "Other version to compare against"},
				"constraint": {Type: "string", Desc: "Constraint to check (e.g. >=1.2.0, <2.0.0, ^1.2.3, ~1.2.3)"},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				version := args["version"]
				if version == "" {
					return "", fmt.Errorf("version is required")
				}
				v, err := parseSemver(version)
				if err != nil {
					return "", fmt.Errorf("invalid version: %w", err)
				}

				result := map[string]any{"version": version}

				if args["other"] != "" {
					other, err := parseSemver(args["other"])
					if err != nil {
						return "", fmt.Errorf("invalid other version: %w", err)
					}
					cmp := v.compare(other)
					result["other"] = args["other"]
					result["result"] = cmp
					if cmp < 0 {
						result["description"] = fmt.Sprintf("%s < %s", version, args["other"])
					} else if cmp > 0 {
						result["description"] = fmt.Sprintf("%s > %s", version, args["other"])
					} else {
						result["description"] = fmt.Sprintf("%s == %s", version, args["other"])
					}
				}

				if args["constraint"] != "" {
					satisfied, err := checkConstraint(v, args["constraint"])
					if err != nil {
						return "", fmt.Errorf("invalid constraint: %w", err)
					}
					result["constraint"] = args["constraint"]
					result["satisfied"] = satisfied
				}

				out, _ := json.Marshal(result)
				return string(out), nil
			},
		},
	}
}

type semver struct {
	major, minor, patch int
	prerelease, build   string
}

func (s semver) String() string {
	v := fmt.Sprintf("%d.%d.%d", s.major, s.minor, s.patch)
	if s.prerelease != "" {
		v += "-" + s.prerelease
	}
	if s.build != "" {
		v += "+" + s.build
	}
	return v
}

func (s semver) compare(other semver) int {
	if s.major != other.major {
		return intCmp(s.major, other.major)
	}
	if s.minor != other.minor {
		return intCmp(s.minor, other.minor)
	}
	if s.patch != other.patch {
		return intCmp(s.patch, other.patch)
	}
	// Pre-release versions have lower precedence
	if s.prerelease == "" && other.prerelease != "" {
		return 1
	}
	if s.prerelease != "" && other.prerelease == "" {
		return -1
	}
	if s.prerelease < other.prerelease {
		return -1
	}
	if s.prerelease > other.prerelease {
		return 1
	}
	return 0
}

func intCmp(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func parseSemver(s string) (semver, error) {
	s = strings.TrimPrefix(s, "v")
	var sv semver

	// Split build metadata
	if idx := strings.Index(s, "+"); idx >= 0 {
		sv.build = s[idx+1:]
		s = s[:idx]
	}
	// Split prerelease
	if idx := strings.Index(s, "-"); idx >= 0 {
		sv.prerelease = s[idx+1:]
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return sv, fmt.Errorf("expected X.Y.Z format, got %q", s)
	}
	var err error
	sv.major, err = strconv.Atoi(parts[0])
	if err != nil {
		return sv, fmt.Errorf("invalid major: %w", err)
	}
	sv.minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return sv, fmt.Errorf("invalid minor: %w", err)
	}
	sv.patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return sv, fmt.Errorf("invalid patch: %w", err)
	}
	return sv, nil
}

func checkConstraint(v semver, constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)

	// Caret: ^1.2.3 means >=1.2.3, <2.0.0
	if strings.HasPrefix(constraint, "^") {
		c, err := parseSemver(constraint[1:])
		if err != nil {
			return false, err
		}
		upper := semver{major: c.major + 1}
		return v.compare(c) >= 0 && v.compare(upper) < 0, nil
	}

	// Tilde: ~1.2.3 means >=1.2.3, <1.3.0
	if strings.HasPrefix(constraint, "~") {
		c, err := parseSemver(constraint[1:])
		if err != nil {
			return false, err
		}
		upper := semver{major: c.major, minor: c.minor + 1}
		return v.compare(c) >= 0 && v.compare(upper) < 0, nil
	}

	// Comparison operators
	for _, prefix := range []string{">=", "<=", "!=", ">", "<", "="} {
		if strings.HasPrefix(constraint, prefix) {
			c, err := parseSemver(strings.TrimSpace(constraint[len(prefix):]))
			if err != nil {
				return false, err
			}
			cmp := v.compare(c)
			switch prefix {
			case ">=":
				return cmp >= 0, nil
			case "<=":
				return cmp <= 0, nil
			case ">":
				return cmp > 0, nil
			case "<":
				return cmp < 0, nil
			case "=":
				return cmp == 0, nil
			case "!=":
				return cmp != 0, nil
			}
		}
	}

	// Exact match
	c, err := parseSemver(constraint)
	if err != nil {
		return false, err
	}
	return v.compare(c) == 0, nil
}
