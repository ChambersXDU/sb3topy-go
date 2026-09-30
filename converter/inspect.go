package converter

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type InspectOptions struct {
	Path        string
	SpecmapData []byte
	Target      string
	BlockID     string
}

type InspectionCheck struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type Inspection struct {
	SchemaVersion int               `json:"schemaVersion"`
	Source        string            `json:"source"`
	SourceKind    string            `json:"sourceKind"`
	CanonicalPath string            `json:"canonicalPath,omitempty"`
	PythonPath    string            `json:"pythonPath,omitempty"`
	Graph         InspectionCheck   `json:"graph"`
	Generation    InspectionCheck   `json:"generation"`
	RoundTrip     *InspectionCheck  `json:"roundTrip,omitempty"`
	Extensions    []string          `json:"extensions"`
	Targets       []InspectedTarget `json:"targets"`
	Issues        []InspectionIssue `json:"issues"`
	Blocks        []InspectedBlock  `json:"blocks"`
}

type InspectedTarget struct {
	Index      int               `json:"index"`
	Name       string            `json:"name"`
	IsStage    bool              `json:"isStage"`
	BlockCount int               `json:"blockCount"`
	Scripts    []InspectedScript `json:"scripts"`
	Variables  []InspectedData   `json:"variables"`
	Lists      []InspectedData   `json:"lists"`
}

type InspectedScript struct {
	ID                  string                 `json:"id"`
	Opcode              string                 `json:"opcode"`
	Fields              map[string]interface{} `json:"fields"`
	GeneratedPythonLine int                    `json:"generatedPythonLine,omitempty"`
}

type InspectedData struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type InspectionIssue struct {
	Code                string `json:"code"`
	TargetIndex         int    `json:"targetIndex"`
	Target              string `json:"target"`
	BlockID             string `json:"blockId"`
	Opcode              string `json:"opcode"`
	GeneratedPythonLine int    `json:"generatedPythonLine,omitempty"`
	Message             string `json:"message"`
}

type InspectedBlock struct {
	TargetIndex         int             `json:"targetIndex"`
	Target              string          `json:"target"`
	ID                  string          `json:"id"`
	GeneratedPythonLine int             `json:"generatedPythonLine,omitempty"`
	MarkerKind          string          `json:"markerKind,omitempty"`
	Raw                 json.RawMessage `json:"raw"`
}

// Inspect reads the Scratch source without converting, synchronizing, or writing files.
func Inspect(opts InspectOptions) (*Inspection, error) {
	source, err := filepath.Abs(opts.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("cannot access project %q: %w", source, err)
	}
	report := &Inspection{
		SchemaVersion: 1, Source: source, SourceKind: "workspace",
		Extensions: []string{}, Targets: []InspectedTarget{}, Issues: []InspectionIssue{}, Blocks: []InspectedBlock{},
	}
	var projectJSON []byte
	if !info.IsDir() && strings.EqualFold(filepath.Ext(source), ".sb3") {
		report.SourceKind = "sb3"
		projectJSON, err = readSB3ProjectJSON(source)
	} else {
		workspace, resolveErr := resolveProjectDir(source)
		if resolveErr != nil {
			return nil, resolveErr
		}
		report.CanonicalPath = filepath.Join(workspace, roundTripDirName, roundTripProjectName)
		report.PythonPath = filepath.Join(workspace, "project.py")
		projectJSON, err = os.ReadFile(report.CanonicalPath)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read Scratch source: %w", err)
	}
	var project ProjectJSON
	if err := json.Unmarshal(projectJSON, &project); err != nil {
		return nil, fmt.Errorf("invalid project.json: %w", err)
	}
	var metadata struct {
		Extensions []string `json:"extensions"`
		Targets    []struct {
			Blocks map[string]json.RawMessage `json:"blocks"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(projectJSON, &metadata); err != nil {
		return nil, err
	}
	if metadata.Extensions != nil {
		report.Extensions = metadata.Extensions
	}
	sm, err := LoadSpecMap(opts.SpecmapData)
	if err != nil {
		return nil, err
	}
	report.Graph = inspectionCheck(validateProjectJSON(projectJSON))
	python, generationErr := TranspileProject(projectJSON, sm)
	report.Generation = inspectionCheck(generationErr)
	if report.SourceKind == "workspace" {
		check := inspectionCheck(VerifyRoundTripProject(source, opts.SpecmapData))
		report.RoundTrip = &check
	}
	locations := generatedBlockLocations(python)
	foundTarget, foundBlock := opts.Target == "", opts.BlockID == ""
	for targetIndex, target := range project.Targets {
		if opts.Target != "" && target.Name != opts.Target {
			continue
		}
		foundTarget = true
		detail := InspectedTarget{
			Index: targetIndex, Name: target.Name, IsStage: target.IsStage, BlockCount: len(target.Blocks),
			Scripts: []InspectedScript{}, Variables: inspectedData(target.Variables), Lists: inspectedData(target.Lists),
		}
		blocksMap := make(map[string]*RawBlockData, len(target.Blocks))
		ids := make([]string, 0, len(target.Blocks))
		for id, raw := range target.Blocks {
			ids = append(ids, id)
			if block, ok := ParseRawBlock(raw); ok {
				blocksMap[id] = block
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			block := blocksMap[id]
			location := locations[blockLocationKey{targetIndex, id}]
			if block != nil && block.TopLevel && sm.IsHat(block.Opcode) {
				fields := block.Fields
				if fields == nil {
					fields = map[string]interface{}{}
				}
				detail.Scripts = append(detail.Scripts, InspectedScript{ID: id, Opcode: block.Opcode, Fields: fields, GeneratedPythonLine: location.line})
			}
			if opts.BlockID == id {
				foundBlock = true
				raw := metadata.Targets[targetIndex].Blocks[id]
				report.Blocks = append(report.Blocks, InspectedBlock{targetIndex, target.Name, id, location.line, location.kind, raw})
			}
			if block == nil || location.line == 0 || location.kind == "unmapped" || location.kind == "hat" {
				continue
			}
			code := strings.TrimSpace(transpileSingleBlock(block, blocksMap, sm, "", newMarkerState(&target)))
			issueCode, message := "", ""
			if strings.HasPrefix(code, "pass  # sb3topy:unsupported opcode=") {
				issueCode, message = "unsupported_block", "No Python translation is available for this block. This is a converter limitation, not evidence of a Scratch project bug."
			} else if strings.HasPrefix(code, "pass  # sb3topy:unsupported-input opcode=") {
				issueCode, message = "unsupported_input", "An input has no usable Python translation. Inspect the referenced reporter blocks before changing the Scratch project."
			}
			if issueCode != "" && (opts.BlockID == "" || opts.BlockID == id) {
				report.Issues = append(report.Issues, InspectionIssue{issueCode, targetIndex, target.Name, id, block.Opcode, location.line, message})
			}
		}
		report.Targets = append(report.Targets, detail)
	}
	if !foundTarget {
		return nil, fmt.Errorf("target %q was not found; run inspect without --target to list target names", opts.Target)
	}
	if !foundBlock {
		return nil, fmt.Errorf("block %q was not found in the selected targets", opts.BlockID)
	}
	return report, nil
}

func inspectionCheck(err error) InspectionCheck {
	if err != nil {
		return InspectionCheck{Detail: err.Error()}
	}
	return InspectionCheck{OK: true}
}

func inspectedData(data map[string]interface{}) []InspectedData {
	result := make([]InspectedData, 0, len(data))
	for id, value := range data {
		name := id
		if tuple, ok := value.([]interface{}); ok && len(tuple) > 0 {
			name = fmt.Sprint(tuple[0])
		}
		result = append(result, InspectedData{id, name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func readSB3ProjectJSON(source string) ([]byte, error) {
	archive, err := zip.OpenReader(source)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	var project *zip.File
	for _, file := range archive.File {
		if file.Name != "project.json" {
			continue
		}
		if project != nil {
			return nil, fmt.Errorf("duplicate project.json ZIP entry")
		}
		project = file
	}
	if project == nil {
		return nil, fmt.Errorf("SB3 archive does not contain project.json")
	}
	reader, err := project.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

type blockLocationKey struct {
	target int
	id     string
}
type blockLocation struct {
	line int
	kind string
}

var inspectionMarker = regexp.MustCompile(`^# sb3topy:(hat|block) id=("(?:[^"\\]|\\.)*") opcode="(?:[^"\\]|\\.)*"(?: kind=(\w+))?`)

func generatedBlockLocations(python string) map[blockLocationKey]blockLocation {
	locations := make(map[blockLocationKey]blockLocation)
	target := -1
	for index, line := range strings.Split(python, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# sb3topy:target ") {
			target++
		}
		match := inspectionMarker.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		id, err := strconv.Unquote(match[2])
		if err != nil {
			continue
		}
		kind := match[3]
		if match[1] == "hat" {
			kind = "hat"
		}
		locations[blockLocationKey{target, id}] = blockLocation{index + 1, kind}
	}
	return locations
}
